# Research: Pre-Build Node Version Existence Check

**Feature**: 017-prebuild-node-check | **Date**: 2026-07-10

Sources: nvm v0.40.5 source (pinned tag kekkai installs), live nodejs.org dist index, `internal/runtime/up.go`, feature-014 artifacts (deleted `baseImageMissing` probe, in-build wrap).

## R1. What nvm actually consults (source-of-truth verification)

- **Finding**: `nvm install <pattern>` resolves remote versions via `nvm_ls_remote_index_tab` (nvm.sh:1669), which downloads `${MIRROR}/index.tab` where MIRROR defaults to `https://nodejs.org/dist` (nvm.sh:2257, `NVM_NODEJS_ORG_MIRROR`). Patterns get a `v` prefix (`nvm_ensure_version_prefix`) and filter the index list.
- **Decision**: Check against `https://nodejs.org/dist/index.json` — same directory and dataset as `index.tab`, JSON-shaped (verified live: 200, ~320KB, array of `{"version":"vX.Y.Z", ...}`).
- **Rationale**: Validating against the installer's own dataset means the check can never disagree with what the build would resolve (FR-001). JSON parses with stdlib; `index.tab` would need line/column splitting for zero benefit.
- **Alternatives considered**:
  - `index.tab` literally — identical data, more parsing code.
  - `HEAD https://nodejs.org/dist/v22.11.0/` per-version — works only for full pins; major/minor selectors need the index anyway; two mechanisms instead of one.
  - GitHub nodejs/node tags — a proxy again; rejected for the same reason the Docker Hub probe was retired.

## R2. Matching rule (mirror nvm exactly)

- **Decision**: full `x.y.z` → index contains exactly `v<sel>`; `x` / `x.y` → index contains any version starting `v<sel>.`. `lts` → no check (FR-002).
- **Rationale**: FR-003. nvm prefix-matches the `v`-prefixed pattern against the version column; `22` matches `v22.*`, `22.11` matches `v22.11.*`. The dot-suffix in the prefix test prevents `2` matching `v22.x` — same boundary nvm's pattern matching has. `lts` (`--lts` at install) always resolves to the current LTS, which by definition exists.
- **Alternatives considered**: Resolving the concrete newest match and logging it — extra behavior with no requirement behind it; the check answers exists/not-exists only.

## R3. Tri-state semantics and shape

- **Decision**: `nodeVersionMissing(selector string) bool` — true ONLY when the index was fetched (HTTP 200), parsed, and no version matched. Timeout (10s, no retries), transport error, non-200, malformed JSON → false (build proceeds, silent). Called only inside the image-miss branch of `ensureImage`, before `buildImage`.
- **Rationale**: FR-004/005/006/007. Identical contract to the 014-era `baseImageMissing` ("only a confirmed 404 aborts; anything inconclusive must never block offline use") — the constitution rationale is unchanged, only the source of truth improved. Placement inside the miss branch makes the cached-image guarantee structural rather than conditional.
- **Alternatives considered**:
  - Erroring on inconclusive with a hint — punishes flaky networks; rejected by spec.
  - Warning line on inconclusive — noise on the happy path of offline/degraded use; FR-005 says silent.
  - Config-validation placement — validation is offline one-pass by constitution; cached runs would also pay network cost.

## R4. Abort error text

- **Decision**: `image.node_version: "<value>" matches no published Node version — see https://nodejs.org/dist/ for available versions`
- **Rationale**: FR-004/FR-009: names the key, the value, and where the list lives. Points at the human-browsable dist listing rather than `nvm ls-remote` (which users can't run until a sandbox exists — chicken-and-egg).
- **Alternatives considered**: Suggesting `nvm ls-remote` — misleading pre-sandbox; naming index.json — machine endpoint, less readable.

## R5. Wrapper removal and the fallback path

- **Decision**: The nvm install RUN becomes `nvm install {{.NodeInstallArg}} && nvm alias default ... && ln -s ...` with no `|| { echo ...; exit 1; }`. A Dockerfile comment states the host-side pre-check handles nonexistent versions and nvm's native error (`Version 'X' not found - try nvm ls-remote`) is the fallback when that check was inconclusive. `NodeVersionRaw` leaves the template data struct (only consumer was the wrap).
- **Rationale**: FR-008/FR-009 and US3: the wrap double-reported (nvm's error + kekkai's echo, two messages disagreeing about the fix) and is now redundant for every reachable-index case. Removing the field keeps render data minimal.
- **Rebuild note**: the template change alters every rendered Dockerfile → hash change → one-time rebuild on next `up` (spec FR-008 note; same expectation as features 014/015).

## R6. E2E strategy, incl. the unreachable-index case

- **Decision**: (a) nonexistent versions (`99`, `22.99`, `22.99.0`) → abort in seconds, zero docker build activity (assert no new `kekkai:*` image, error text per contract); (b) valid `22` + `lts` → build proceeds (and `lts` provably skips the check — verify via absence of any index request, e.g. run with index host blocked and `lts`: still instant-proceed); (c) inconclusive: block nodejs.org resolution for the probe (e.g. `https_proxy` pointing at a dead port for the kekkai process) with a valid version → build proceeds silently; (d) native-error fallback: same blocking trick with a bad version → build fails showing only nvm's message.
- **Rationale**: Constitution IV — real runs, real index, real builds. The proxy-env trick makes the probe fail without touching system network (the docker build itself uses the daemon and is unaffected — which is exactly the split the edge case describes).
- **Alternatives considered**: hosts-file edits (system modification, needs root) — rejected; mock index server — violates e2e principle for the abort path, but the proxy trick keeps even the inconclusive path real-binary.
