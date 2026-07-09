# Research: Debian Base Image with Build-Time Node Install (nvm)

**Feature**: 014-debian-nvm-base | **Date**: 2026-07-09

All unknowns from Technical Context resolved. Sources: codebase reads (`embed/Dockerfile.tmpl`, `internal/config/*.go`, `internal/runtime/up.go`, `SPECIFICATION.md`), nvm project documentation, GitHub API (release lookup).

## R1. Which nvm release to pin

- **Decision**: `v0.40.5` as a code constant (alongside the `debian:trixie` constant).
- **Rationale**: Latest tagged release (published 2026-06-04, verified via GitHub API on 2026-07-09). FR-003 forbids `master`/`latest`; a tag makes the installer URL and script content reproducible, and the tag string enters the rendered Dockerfile, so bumping it changes the image hash — exactly the bake-time-input behavior the constitution requires.
- **Alternatives considered**:
  - Pinning by commit SHA — stronger immutability but unreadable in diffs and no easier to audit; tags on nvm-sh are release-managed and sufficient.
  - Vendoring the install script into `embed/` — fully offline-reproducible, but adds a maintained third-party file to the repo (surface + update burden) for no user-visible gain; rejected under Minimal Surface.

## R2. Selector translation to nvm install arguments

- **Decision**: `lts` → `nvm install --lts`; `22`, `22.11`, `22.11.0` pass through verbatim as `nvm install <selector>`.
- **Rationale**: `--lts` is nvm's documented "latest LTS" selector (FR-004's "installer's latest-LTS selector"). nvm natively resolves partial versions (`22` → newest 22.x.y, `22.11` → newest 22.11.x), so no version arithmetic lands in kekkai. `nvm alias default <same selector>` plus `nvm use` make it the default node.
- **Alternatives considered**:
  - `lts/*` passthrough — same meaning, but keeping the user-facing value (`lts`) distinct from nvm syntax lets kekkai swap installers later without a schema change.
  - Resolving version → concrete x.y.z on the host before build (via nodejs.org index) — would let `lts` bumps trigger rebuilds, but the spec explicitly freezes `lts` at build time and forbids new network dependencies at parse time; rejected.

## R3. Validation pattern for `image.node_version`

- **Decision**: Replace `nodeVersionPattern` (`^[a-z0-9.]+$`) with `^(lts|[0-9]+(\.[0-9]+){0,2})$` in `internal/config/validate.go`; error message enumerates the accepted forms: `lts`, `22`, `22.11`, `22.11.0`.
- **Rationale**: FR-004 requires rejecting installer aliases (`node`, `stable`, `lts/*`, `lts/<codename>`) at parse time with the accepted forms named. The old pattern existed to match node:* Docker tags (`current`, codenames) — those are now invalid. The empty-string check and the absent-key→`lts` default already exist and are kept (`config.go: applyDefaults`, `validate.go` empty check).
- **Alternatives considered**: Accepting `current` for continuity — rejected: spec tightens the value set deliberately; `current` on nvm (`node` alias) is a moving non-LTS target the spec excludes.

## R4. Making node/npm/npx work from every exec path

- **Decision**: Two mechanisms, both baked at build time:
  1. During the nvm install RUN (as `kekkai`), create a stable symlink `/home/kekkai/.nvm/current` → the resolved `versions/node/vX.Y.Z` directory.
  2. As root, symlink `/home/kekkai/.nvm/current/bin/{node,npm,npx}` and `claude` into `/usr/local/bin` (FR-005/FR-006), and set `ENV PATH=/home/kekkai/.nvm/current/bin:$PATH`.
  Build steps that need nvm use the documented non-interactive pattern: `SHELL ["/bin/bash", "-c"]` with `BASH_ENV` pointing at a file that sources `nvm.sh` (FR-005).
- **Rationale**: `docker exec` and `sh -c` get the image `ENV PATH` but never source profiles; `/usr/local/bin` symlinks additionally survive PATH resets by user tooling. The `current` indirection keeps symlinks version-agnostic and — because PATH includes the live nvm bin dir — binaries added later by runtime `npm install -g` are reachable from all exec paths too. The nvm dirs stay wholly owned by `kekkai` (FR-008); only the `/usr/local/bin` symlinks are root-created, and symlinks don't affect target ownership.
- **Alternatives considered**:
  - Only `ENV PATH` without symlinks — spec mandates the symlink guarantee (FR-005), and PATH alone breaks under `env -i` / hardcoded-PATH scripts.
  - Copying binaries instead of symlinking — breaks npm's relative resolution of its own module tree; rejected.
  - `n` or plain tarball install to `/usr/local` — root-owned `node_modules` would make runtime `npm install -g` fail without sudo (violates FR-008/US1-4); nvm's per-user layout is the point.

## R5. NPM_CONFIG_PREFIX conflict

- **Decision**: Drop `NPM_CONFIG_PREFIX=/usr/local/share/npm-global` and the shared prefix dir entirely. Claude Code installs into the nvm-managed global dir.
- **Rationale**: nvm is explicitly incompatible with a set npm `prefix` (it errors out / demands `nvm use --delete-prefix`), so the existing prefix scheme cannot coexist with nvm. The nvm global dir is user-owned, which is exactly FR-008's requirement; the shared-prefix machinery existed only because the old base's globals were root-owned.
- **Alternatives considered**: Keeping the prefix and un-setting it around nvm calls — fragile (any runtime shell with the env var re-breaks npm/nvm interplay); rejected.

## R6. Surfacing the resolved Node version at startup (FR-012)

- **Decision**: The image CMD prints one line before firewall init: `kekkai sandbox: node $(node -v), claude <rendered claude version>`. No host-side docker calls added.
- **Rationale**: The concrete x.y.z is only known inside the image (nvm resolved it at build). Printing from CMD costs nothing, shows on **every** `kekkai up` (SC-007), and is correct even on the §6.2 offline-fallback path where the host doesn't know the claude version (it's baked into the reused image's CMD). Host-side alternatives all need an extra `docker run`/`create` round-trip per `up`, violating the no-new-docker-calls constraint.
- **Alternatives considered**:
  - `docker run --rm <tag> node -v` after ensureImage — extra container start on every `up`; rejected.
  - Baking a `LABEL` with the version — LABEL values are fixed at render time; the resolved version isn't known until a RUN step executes. BuildKit metadata tricks are daemon-version-sensitive; rejected.

## R7. Failing builds that name the config knob (edge case)

- **Decision**: The `nvm install` step wraps failure: `nvm install <sel> || { echo "ERROR: Node version <sel> not found - check image.node_version in .kekkai.yaml"; exit 1; }`.
- **Rationale**: Well-formed-but-nonexistent versions (`99`, `22.99.0`) can't be caught at parse time (spec assumption); the wrap makes the last lines of build output point at `image.node_version` instead of reading as an internal Dockerfile bug.
- **Alternatives considered**: Host-side pre-check against the nodejs.org version index — new network dependency and duplicate of nvm's own resolution; rejected (spec assumption says surface at build time).

## R8. Obsolete Docker Hub tag probe

- **Decision**: Delete `baseImageMissing` (up.go) and its anonymous-token/manifest-HEAD handshake; remove the corresponding §6.1 paragraph from SPECIFICATION.md.
- **Rationale**: The probe existed to pre-validate `node:<tag>` existence. The base is now a kekkai constant (`debian:trixie`) that always exists; user-driven version errors moved to build time (R7). Net −45 lines, Minimal Surface win.
- **Alternatives considered**: Repointing the probe at `library/debian` — pointless (constant tag, kekkai-owned; a rename breaks CI long before users see it).

## R9. User creation and UID/GID

- **Decision**: `groupadd -g 1000 kekkai && useradd -m -u 1000 -g kekkai -s /bin/zsh kekkai` replaces the `usermod` rename of the node user.
- **Rationale**: `debian:trixie` ships no non-root user. The old base's `node` user was UID/GID 1000; creating `kekkai` at 1000 keeps workspace file ownership behavior byte-identical (FR-007). zsh is installed by the builtin apt set before user creation.
- **Alternatives considered**: Build-arg host-UID mapping — new feature territory (changes image hash per host), out of scope; today's behavior is fixed 1000.

## R10. Baseline apt packages on bare Debian

- **Decision**: Add `bash` to `builtinAptPackages` (FR-002 names it); `curl`, `ca-certificates`, `git`, `procps` are already in the builtin set. Keep the existing GitHub CLI apt-repo bootstrap (gh is not in Debian repos).
- **Rationale**: `debian:trixie` includes bash already, so the entry is a no-op install that documents the dependency (nvm requires bash; BASH_ENV pattern requires bash). The early `apt-get install ca-certificates curl` step for the gh keyring stays — it now also serves the nvm installer download.
- **Alternatives considered**: None substantive.

## R11. apt-installed nodejs shadowing (edge case)

- **Decision**: No code change needed; document in the Dockerfile comment. `ENV PATH` puts `/home/kekkai/.nvm/current/bin` first, and `/usr/local/bin` (symlinks) precedes `/usr/bin` (Debian's `nodejs`) in the default PATH — the nvm Node always wins resolution.
- **Rationale**: Both lookup orders guarantee precedence for every exec path; a user adding `nodejs`/`npm` via `image.apt_packages` gets a working but shadowed Debian node.

## R12. Build-network vs firewall (FR-010)

- **Decision**: Dockerfile comment at the nvm/download steps stating build-time downloads (nvm installer from raw.githubusercontent.com, Node tarballs from nodejs.org, npm registry) run on the host/daemon network before the sandbox firewall exists, and must never be added to `network.allowed_domains` or §5.4.
- **Rationale**: FR-010 requires the documentation in code; no functional change — `docker build` has never run behind `init-firewall.sh` (the firewall starts at container CMD).
