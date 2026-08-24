# Quickstart: Verify Claude Binary at Image Build Time

End-to-end validation against a real docker daemon (constitution IV). Contract under test: [contracts/claude-install-verify.md](contracts/claude-install-verify.md).

## Prerequisites

- Built kekkai binary: `go build -o kekkai-test ./cmd/kekkai`
- Docker daemon running; a scratch test project dir (e.g. `/tmp/kekkai-qs-023` with a minimal `.kekkai.yaml`)
- `kekkai up` ends in `docker run -it`: for scenarios needing a live container use the pseudo-TTY wrapper `script -qec "kekkai-test up" /dev/null </dev/null` backgrounded; plain non-TTY runs still exercise everything up to the run handoff. Clean up the test container, its `kekkai-history-*` volume (test pwd hash only), and test-built images afterwards.

## S1 — Happy path: build succeeds, claude runnable (US1 scenario 3, SC-004)

1. In the test project, remove any image for the current config, then run `kekkai up` (non-TTY is fine — build completes before the TTY error).
2. Expect: build succeeds; the install step's log (visible with `--verbose`) shows `npm install` followed by one `claude --version` version line; no retry notice, no failure message.
3. `docker run --rm kekkai:<newtag> claude --version` → exits 0, prints the resolved version.
4. Compare build duration against a pre-feature build: overhead ≤ a few seconds (expected ~0.1s).

## S2 — Broken install aborts the build, message shown, no tag (US1 scenarios 1-2, FR-001..003)

Simulate npm's silent optional-dependency drop deterministically with `--ignore-scripts` (postinstall never copies the native binary over the `bin/` placeholder — the same non-functional state the transient drop produces; note `--omit=optional` does NOT work as a simulation: npm 11.17 installs the platform packages regardless, verified 2026-08-24):

1. Render/copy the current Dockerfile (kekkai stages it in `/tmp/kekkai-build-*/Dockerfile` during a build; copy it mid-build together with `embed/init-firewall.sh` as context) and change **both** install commands in the step to append `--ignore-scripts`.
2. `docker build -t kekkai-qs-023-broken <dir>`.
3. Expect: first `claude --version` fails → retry notice line → second install (also omitting) → second verify fails → stderr line containing `claude native binary failed to install` and `rerun 'kekkai up'` → build exits nonzero.
4. `docker image inspect kekkai-qs-023-broken` → must fail (no tag created).
5. Repeat the build **without** `--progress=plain` in an interactive terminal: the failing step's output (incl. the message) must still be printed by BuildKit's compact renderer.

## S3 — Retry actually re-downloads (US2, FR-004)

The double-`--ignore-scripts` build in S2 already proves the retry executes (its notice line appears and a second npm run follows). For retry-success, verify the underlying property the contract relies on:

1. `docker run --rm --entrypoint bash <broken-2.1.x-image-or-S2-equivalent> -c 'npm install -g @anthropic-ai/claude-code@<ver> && claude --version'`
2. Expect: the re-install fetches the ~342MB platform package and `claude --version` exits 0 — an identical re-install heals the missing optional dependency (validated 2026-08-24 against `kekkai:cabb8e08f13d`).

## S4 — One-time rebuild for existing users; hash semantics (US3 scenario 1, FR-006)

1. With a pre-feature image present for the test config, run post-feature `kekkai up`.
2. Expect: `building image kekkai:<newhash> …` (template text changed → new hash), exactly once; a second `up` reuses `kekkai:<newhash>` with no build.
3. `docker image inspect` both images: `kekkai.config_hash` labels are **identical** (formula untouched).

## S5 — Offline fallback unchanged (US3 scenario 2, FR-007)

1. With `claude.version: latest` and at least one existing matching image, make the npm registry unreachable for the resolver (e.g. temporary hosts-file block of `registry.npmjs.org` for the host process, or run disconnected).
2. Run `kekkai up`.
3. Expect: the pre-existing warning `npm registry unreachable … reusing existing image <tag>` and reuse of the newest config-hash-matching image — no build, no new verification-related output or failure mode.

## Expected outcomes summary

| Scenario | Pass condition |
|----------|----------------|
| S1 | build OK, in-image `claude --version` exit 0, negligible overhead |
| S2 | build fails, message with cause + `rerun 'kekkai up'` remedy, no tag, message visible in compact progress |
| S3 | identical re-install heals the broken state (retry mechanism sound) |
| S4 | exactly one rebuild after upgrade; config_hash label unchanged |
| S5 | offline fallback byte-identical to pre-feature behavior |
