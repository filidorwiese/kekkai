# Research: Custom Apt Repositories (`image.apt_repos`)

All Technical Context unknowns resolved. Decisions below; alternatives noted where a real choice existed.

## R1. Registration mechanism: one-line deb entries in the existing RUN block

**Decision**: For each entry, emit (a) an optional keyring download to `/etc/apt/keyrings/kekkai-<name>.gpg` and (b) a one-line `deb ...` entry written to `/etc/apt/sources.list.d/kekkai-<name>.list`. Both go into the existing first RUN block of `embed/Dockerfile.tmpl`, after the builtin GitHub CLI repo setup and before the final `apt-get update && apt-get install`, so user packages resolve from user repos in the same build (FR-007) and layer caching behavior is unchanged (still one apt layer).

**Rationale**: Mirrors the builtin GitHub CLI pattern exactly (curl to keyrings dir, echo a deb line, no gpg tooling); `curl` and `ca-certificates` are already installed at that point in the block. The `kekkai-` filename prefix makes filesystem collision with `github-cli.list` / `githubcli-archive-keyring.gpg` impossible regardless of the user's `name` (FR-009).

**Alternatives considered**: deb822 `.sources` files — explicitly out of scope per feature description; separate RUN layer per repo — more layers, no benefit, breaks the "one apt layer" shape.

## R2. Key handling without gpg: apt auto-detects armored keys

**Decision**: Download the key verbatim (`curl -fsSL <key_url> -o .../kekkai-<name>.gpg`, then `chmod 0644`) and reference it via `signed-by=`. No dearmoring step.

**Rationale**: apt ≥ 2.4 (trixie ships 2.9/3.x) auto-detects ASCII-armored vs binary OpenPGP keys in `signed-by` files, so both common vendor key formats work (spec edge case) without adding `gnupg` to the image. Same trick the builtin GitHub CLI entry relies on ("the keyring file needs no gpg tooling").

**Alternatives considered**: `gpg --dearmor` — requires installing gnupg first, bigger surface, ordering complexity; rejected.

## R3. deb line options: always `arch=`, `signed-by=` only with key_url

**Decision**:
- With `key_url`: `deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/kekkai-<name>.gpg] <url> <suite> <components>`
- Without `key_url`: `deb [arch=$(dpkg --print-architecture)] <url> <suite> <components>`

**Rationale**: `arch=` matches the builtin entry and prevents foreign-arch index fetches on multi-arch-capable repos. Omitting `signed-by` makes apt fall back to `/etc/apt/trusted.gpg.d` — where `debian-archive-keyring` installs the Debian archive keys in trixie — which is exactly the spec's "keys Debian already ships" case (backports). No `trusted=`/other options are ever emitted, and validation makes them inexpressible in field values (FR-004).

**Alternatives considered**: requiring `key_url` always — breaks the backports use case; skipping `arch=` — works but diverges from the builtin pattern and invites "skipping acquire of configured file" noise from single-arch vendor repos.

## R4. Flat repositories

**Decision**: `suite` ending in `/` (e.g. `./`, `debian/`) marks a flat repo (clarification 2026-08-20). Rendered line: `deb [<options>] <url> <suite>` — no components token. Validation rejects a set `components` on flat entries and rejects `/` in non-flat suites.

**Rationale**: This is apt's own flat-repo grammar; supporting it costs one conditional in the template data and one validation rule, and covers vendor repos that only publish flat layouts.

## R5. Injection defense: per-field allowlist regexes

**Decision**: Validation is allowlist-only (never blocklist), applied at config load in `Validate()`:

| Field | Rule |
|---|---|
| `name` | `^[a-z0-9-]{1,64}$`; unique in list; not a reserved builtin name (`github-cli`, `githubcli`) |
| `url`, `key_url` | must start `https://`; remainder matches `^[A-Za-z0-9._~%/+:-]+$` |
| `suite` (non-flat) | `^[A-Za-z0-9._+-]+$` |
| `suite` (flat, ends `/`) | `^(\./|([A-Za-z0-9._+-]+/)+)$` (path segments, each slash-terminated; no `..` possible since `/` only terminates segments and a segment of only dots is rejected via an explicit `..` ban) |
| `components` | `^[a-z0-9-]+$` (single component); must be absent on flat entries |

**Rationale**: Values are interpolated by Go `text/template` into a shell `RUN` line and a double-quoted `echo` — the allowlists exclude whitespace, quotes, backslash, backtick, `$`, `;`, `&`, `|`, `<`, `>`, `(`, `)`, `{`, `}`, `[`, `]`, `=`, `#`, `?`, `!`, `*` and every other shell/apt-syntactic character. Because `[`, `]` and `=` are unrepresentable, apt option syntax (`trusted=yes`) cannot be smuggled through any field (FR-004) — a structural guarantee, not a pattern match for the word "trusted". `%` stays allowed in URLs for percent-encoding; `:` for ports.

**Alternatives considered**: URL-parse via `net/url` plus scheme check only — accepts query strings, userinfo, and shell-hostile characters; rejected in favor of parse *and* charset allowlist (belt and suspenders: scheme check via prefix, charset via regex).

## R6. Image identity and rebuilds

**Decision**: No change needed for `ImageTag` — repos alter the rendered Dockerfile, and `ImageTag = sha256(rendered + firewall)` already keys builds (FR-008 for free). `ConfigHash` (the §6.2 offline-fallback label, computed before rendering) gains one input: the repo list serialized deterministically in config order as `name|url|suite|components|key_url` lines. Zero entries serialize to the empty string → identical hash to today. The template's `{{range .AptRepos}}` block uses `{{- -}}` whitespace control so an empty list renders a byte-identical Dockerfile → existing images stay valid, no rebuild churn (spec edge case: `[]` ≡ absent).

**Rationale**: Follows the existing split exactly: `ImageTag` keys builds, `ConfigHash` keys only the offline fallback and must cover every bake input minus the claude version — without repos in it, the fallback could reuse an image baked for different repos.

## R7. NO_PUBKEY hint

**Decision**: `docker.BuildImage` tees the build's stdout/stderr through an `io.MultiWriter` (terminal + buffer) and returns the captured output alongside the error. On build failure with `apt_repos` configured, `runtime` scans the output for apt signature-failure markers (`NO_PUBKEY`, `is not signed`, `EXPKEYSIG`, `NODATA`) and attributes the failure by matching each entry's `url` against the error lines (apt names the repo URL in its release-file errors). Hint format (stderr, after the build error): names the entry, then — if `key_url` set — "check image.apt_repos[i] (<name>): key_url may be the wrong key"; if unset — "add key_url to image.apt_repos[i] (<name>)". If no URL matches, a generic one-line hint lists the configured repo names.

**Rationale**: Keeps the docker package dumb (capture only, no apt knowledge) and the hint logic next to the config it references. Attribution by URL is reliable because apt's `The repository '<url> <suite> ...' is not signed` / `NO_PUBKEY` lines always carry the source URL.

**Alternatives considered**: parsing the missing key ID and mapping to repos — impossible without fetching keys ourselves; pre-flight key validation on the host — new host-side network behavior and a second source of truth, rejected.

## R8. Defaults handling for `components`

**Decision**: Leave `components` empty in the parsed config; `renderDockerfile`'s per-repo data substitutes `main` for empty on non-flat entries. Validate treats non-flat empty as "defaulted", flat non-empty as an error.

**Rationale**: A per-entry key-presence tracker (the `imageKeysSet` pattern) just to distinguish `components: ""` from absent is not worth it; explicit empty string behaving as the default is harmless and documented. Mirrors how `NodeInstallArg()` maps config values at render time.

## R9. Documentation targets

**Decision**: (a) `starterConfig` in `internal/runtime/init.go`: commented `apt_repos` block using the Dart example from the feature description, placed under `image:` next to `apt_packages` (follows the `apt_packages: [golang]` precedent — illustrative example, not a default). (b) README image section: same example plus field table. (c) SPECIFICATION.md: §4.2 schema block, §4.4 validation bullets, §6.1 bake-time inputs sentence, §6.3 Dockerfile contract (registration lines + host-network download note already covers build-time fetches).
