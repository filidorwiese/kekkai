# Quickstart: validating quiet build progress end-to-end

Constitution IV: e2e against a real docker daemon. Renderer checks need docker to see a terminal — run `up` through a pseudo-TTY and inspect the transcript (research.md R5). Builds only need to run long enough to emit their progress header; kill with `timeout` where noted.

## Prerequisites

```sh
go build -o kekkai-test ./cmd/kekkai        # from repo root
mkdir -p /tmp/kekkai-quiet && cd /tmp/kekkai-quiet
```

Classification markers in a transcript: **compact** ⇔ contains `[+] Building` and NO `#1 [internal] load build definition` step lines; **plain** ⇔ step lines present.

## S1 — Quiet interactive build without repos (US1; SC-001)

No `.kekkai.yaml` (or one without `apt_repos`), and no cached image (vary `apt_packages`, e.g. `[sl]`, to force a build). Run ~20 s through a pty and inspect:

```sh
timeout 20 script -qec "../path/to/kekkai-test up" /dev/null > s1.log 2>&1
grep -c '\[+\] Building' s1.log            # >= 1
grep -c '#1 \[internal\] load build' s1.log # 0
```

Expect: compact markers, zero plain step lines.

## S2 — Hint regression gate (US2; SC-002)

Re-run specs/020 quickstart S5 exactly: Dart repo config **without** `key_url`, `apt_packages: [dart]`. Run `up` (non-TTY fine; fails at `apt-get update`, ~60–90 s). Expect: NO_PUBKEY in output, hint line `hint: image.apt_repos[0] (dart): … add key_url …` on stderr after the build error, exit non-zero. Also confirm this repo build's transcript is plain (capture mode) — step lines present.

## S3 — Verbose forces plain in both modes (FR-004; SC-004)

Repeat S1's pty run with `up --verbose` (force a build with another `apt_packages` variant): expect plain step lines even on a terminal. The S2 run with `--verbose` behaves identically to today.

## S4 — Zero rebuilds from the fix (FR-005; SC-003)

With any image built by the pre-fix binary present (e.g. the current defaults image): run the fixed binary's `up` with the same config — expect no `building image` line (tag reused). `ConfigHash` label of a freshly built image equals the pre-fix binary's for the same config.

## S5 — No hint without repos (FR-003)

Force a build failure without `apt_repos` (e.g. `apt_packages: [kekkai-no-such-package-xyz]`): build fails, exit non-zero, zero `hint:` lines on stderr.

## S6 — Sweep

`go vet ./...`, `go build ./cmd/kekkai`. Firewall untouched by inspection (no changes near it), standard probes observed during any live run.
