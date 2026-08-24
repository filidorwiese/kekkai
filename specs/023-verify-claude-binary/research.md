# Research: Verify Claude Binary at Image Build Time

All findings verified empirically against real images/containers on 2026-08-24 (docker daemon, images `kekkai:cabb8e08f13d` = broken 2.1.241 install, `kekkai:0a0179ef2d20` = healthy install). Root-cause evidence: npm debug log baked into the broken image at `/home/kekkai/.npm/_logs/2026-08-24T06_26_53_269Z-debug-0.log` shows `verbose reify failed optional dependency …claude-code-linux-x64` — the ~342MB platform tarball fetch failed transiently; npm exited 0 anyway (optionalDependency semantics), the wrapper's postinstall also exited 0, and the broken image was hash-tagged and reused by every subsequent `up`.

## R1: Verification mechanism

- **Decision**: `claude --version` appended to the install RUN step (`npm install -g … && claude --version`).
- **Rationale**: The wrapper's `install.cjs` copies the native binary from the platform optionalDependency over the `bin/` placeholder; `claude` then execs the native binary directly. `--version` therefore proves exactly the property that failed: native binary present *and runnable*. Verified: broken image → exit 1 with "Error: claude native binary not installed"; healthy image → exit 0. Fail-safe per spec edge case: any future distribution change that leaves `claude` unrunnable also fails the check, regardless of mechanism. Works inside the RUN step without the later `/usr/local/bin` symlinks: the nvm-sourced shell (`BASH_ENV`) has the nvm bin dir — where npm -g links `claude` — on PATH.
- **Alternatives considered**: (a) `test -x` on the native package path under `node_modules/@anthropic-ai/claude-code-linux-*` — rejected: encodes today's package layout, breaks or silently weakens when Anthropic changes distribution; existence ≠ runnable. (b) Running `node install.cjs` and checking output — rejected: postinstall exits 0 even when the package is absent (that exit-0 is the bug's second half). (c) Host-side post-build `docker run <tag> claude --version` in Go — rejected: image is already tagged by then, so a failure would need untag/cleanup logic (FR-002 free with in-build failure), plus new Go surface (Principle III).

## R2: Retry semantics

- **Decision**: On first verification failure, re-run the identical `npm install -g @anthropic-ai/claude-code@<version>` once, then verify again. No `npm cache clean`, no `--force`.
- **Rationale**: FR-004 wants exactly one retry. Re-running the same install re-reifies the tree: npm sees the optional dependency missing on disk and re-attempts the download. Verified: executing the same `npm install -g` inside the broken image installed the 342MB native package and produced a working `claude` (this is how the root cause was confirmed). The failed first fetch leaves no poisoned cache entry — the build log shows the tarball was never cached (`no local data … Extracting by manifest` followed by fetch failure), so the retry genuinely re-downloads (spec edge case satisfied).
- **Alternatives considered**: (a) `npm cache clean --force` before retry — rejected: cache is empty on the failure path (fresh layer, failed fetch not cached); adds time and noise for nothing. (b) Configurable retry count — rejected: Principle III, no new config surface; transient failures either clear on one retry or the user reruns `up`. (c) npm-level knobs (`--fetch-retries`) — rejected: npm already retries at HTTP level; the observed failure got through those, so the meaningful retry is a full re-reify.

## R3: Surfacing the failure to the `kekkai up` user

- **Decision**: The RUN step itself echoes one actionable line to stderr before exiting nonzero: cause (claude native binary failed to install — usually a transient npm registry/CDN failure) and remedy (rerun `kekkai up` to retry the build). No Go-side changes.
- **Rationale**: BuildKit prints the failing step's captured output in **all** progress modes (compact TTY, plain, and the specs/022 non-captured mode), so the message reaches the user regardless of whether output capture is active. `up` already propagates `docker build failed` as a hard error. Extending the specs/020 host-side hint machinery is unnecessary: that exists because apt's markers needed attribution to a config entry; here the build step can speak for itself.
- **Alternatives considered**: host-side marker-scan + hint à la `aptSignatureHint` — rejected: capture is only enabled when `apt_repos` are configured (specs/022), so the hint would be silently absent in the common zero-repo case; the in-Dockerfile echo works everywhere and keeps the change out of Go.

## R4: No broken image may survive a failed verification (FR-002)

- **Decision**: Rely on docker's native behavior — a build whose RUN step exits nonzero produces no tagged image; `docker image inspect <tag>` still misses, so the next `up` rebuilds.
- **Rationale**: Verified semantics of `docker build -t`; nothing to implement. Intermediate build-cache layers up to the failed step are retained by BuildKit, which is desirable: the retry-after-rerun skips apt/nvm/node and re-executes only the claude install step.
- **Alternatives considered**: none needed.

## R5: Hash and offline-fallback interplay (FR-006, FR-007)

- **Decision**: Accept the one-time image-hash change from the edited Dockerfile text; touch nothing else.
- **Rationale**: §6.1: image hash = sha256(rendered Dockerfile + init-firewall.sh) — editing the template is the normal recipe-change path, forcing exactly one rebuild for existing users (spec US3). `kekkai.config_hash` (the §6.2 offline-fallback key) hashes bake inputs minus the Dockerfile text and is untouched, so the fallback keeps matching both pre- and post-feature images. The fallback path never builds, so it cannot hit the new verification (FR-007 holds by construction).
- **Alternatives considered**: retroactive detection of already-broken images (e.g. `docker run <existing> claude --version` before reuse) — rejected: adds a container start to every cached `up` (§7.4 explicitly keeps the happy path free of extra docker calls), and the forced rebuild replaces broken images anyway.

## R6: SPECIFICATION.md amendment (Principle I)

- **Decision**: Amend the §6.3 bullet "claude installed via `npm install -g @anthropic-ai/claude-code@<resolved claude.version>` as `kekkai`" to add: the install is verified with `claude --version` in the same RUN step (the npm wrapper's platform-native optionalDependency can be dropped silently on transient registry failure); one identical re-install retry on failure; double failure aborts the build with an actionable stderr line naming the transient cause and the rerun remedy. Contract reference: `specs/023-verify-claude-binary/contracts/claude-install-verify.md`.
- **Rationale**: Principle I — design change lands in SPECIFICATION.md in the same commit as the template change. No other spec section describes the install step; §6.1/§6.2 text needs no change (R5).
- **Alternatives considered**: none — constitutionally mandated.

## R7: Verification must not need network (FR-005)

- **Decision**: `claude --version` is the check precisely because it is offline-capable.
- **Rationale**: Verified: `docker run --network none … claude --version` → exit 0 in 0.077s on a healthy install. The only network use in the step remains `npm install` itself (already sanctioned by §6.1 build-time downloads). SC-004 overhead: ~0.1s per successful build.
- **Alternatives considered**: covered under R1.
