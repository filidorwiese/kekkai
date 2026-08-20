# Contract: image render, identity, and build-failure hint

Scope: `embed/Dockerfile.tmpl`, `internal/runtime` (render + identity + hint), `internal/docker` (build output capture).

## Rendered Dockerfile lines

Inside the existing first RUN block, after the builtin GitHub CLI repo setup and **before** the final `apt-get update && apt-get install`, per entry in config order:

With `key_url`:

```
 && curl -fsSL <key_url> -o /etc/apt/keyrings/kekkai-<name>.gpg \
 && chmod 0644 /etc/apt/keyrings/kekkai-<name>.gpg \
 && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/kekkai-<name>.gpg] <url> <suite-line>" \
      > /etc/apt/sources.list.d/kekkai-<name>.list \
```

Without `key_url`:

```
 && echo "deb [arch=$(dpkg --print-architecture)] <url> <suite-line>" \
      > /etc/apt/sources.list.d/kekkai-<name>.list \
```

`<suite-line>`: non-flat ⇒ `<suite> <components>` with components defaulting to `main`; flat (suite ends `/`) ⇒ `<suite>` alone, no components token.

Guarantees:

- Filenames always carry the `kekkai-` prefix ⇒ can never collide with the builtin `github-cli.list` / `githubcli-archive-keyring.gpg` (FR-009).
- No option other than `arch=` and (conditionally) `signed-by=` is ever emitted (FR-004/FR-006).
- The builtin GitHub CLI lines are byte-identical to today.
- **Empty `apt_repos` renders a byte-for-byte identical Dockerfile to the pre-feature template** (template whitespace control), so existing images keep their tags and no rebuild is triggered by upgrading kekkai. This is testable: render with zero repos and compare against the current template's output.

## Identity

- `ImageTag`: unchanged formula `sha256(rendered Dockerfile + init-firewall.sh)[:12]` — any repo change alters the rendered text, changing the tag ⇒ rebuild on next `up` (FR-008). Unchanged config ⇒ same tag ⇒ no rebuild.
- `ConfigHash`: gains one appended input — the repo list serialized in config order as `name|url|suite|components|key_url` lines (raw values, no defaulting). Zero entries contribute the empty string, keeping today's hashes valid. SPECIFICATION.md §6.1 label formula updated accordingly.

## Build output capture and signature hint

- `docker.BuildImage` writes build output to the terminal as today **and** captures it (`io.MultiWriter`), returning the captured text with the error. Success path behavior unchanged.
- On build failure, when `image.apt_repos` is non-empty and the captured output matches any of `NO_PUBKEY`, `is not signed`, `EXPKEYSIG`, `NODATA`, `runtime` prints one hint line to stderr after the build error:
  - Attribution: first entry whose `url` appears in the captured error text.
    - Entry has `key_url`: `hint: image.apt_repos[<i>] (<name>): apt could not verify this repository — key_url may point at the wrong key`
    - Entry lacks `key_url`: `hint: image.apt_repos[<i>] (<name>): apt could not verify this repository — add key_url with the repository's signing key`
  - No entry's URL matches: `hint: an apt repository failed signature verification — check key_url on image.apt_repos entries (<name list>)`
- The hint never suppresses or replaces the underlying build error; exit status unchanged.
- Builds without `apt_repos` never print the hint, whatever the failure.
