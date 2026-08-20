# Quickstart: validating builtin tools end-to-end

Constitution IV: validation is e2e against a real docker daemon. Build the binary, run in a scratch project. Non-TTY `kekkai up` exits after the build ("the input device is not a TTY") — fine except where a live sandbox is needed; use the pseudo-TTY pattern (`script -qec "kekkai-test up" /dev/null </dev/null &`, poll `docker ps`) for in-sandbox checks via `kekkai-test exec`.

## Prerequisites

```sh
go build -o kekkai-test ./cmd/kekkai        # from repo root
mkdir -p /tmp/kekkai-tools && cd /tmp/kekkai-tools   # NO .kekkai.yaml — defaults on purpose
```

Cleanup after each run: `docker rm -f <container>`, remove only this project's `kekkai-history-<hash>` volume. Expect one ~2–4 min build.

## S1 — Tools present under canonical names (US1+US2; SC-001)

No config file. Run `up` (build), then start a live sandbox and check every tool:

```sh
for t in unzip zip xz zstd bzip2 file rg fd fdfind rsync; do kekkai-test exec sh -c "command -v $t && $t --version | head -1 || $t -v"; done
```

Expect: all ten resolve (`fd` → `/usr/local/bin/fd`, `fdfind` → `/usr/bin/fdfind`) and print a version. `rg` and `fd` also return real matches against `/workspace`.

## S2 — Archive round-trips (US1; SC-002)

In the same sandbox:

```sh
kekkai-test exec sh -c 'cd /tmp && echo hello > a.txt \
  && zip -q r.zip a.txt && rm a.txt && unzip -q r.zip && grep -q hello a.txt \
  && tar cJf r.txz a.txt && rm a.txt && tar xJf r.txz && grep -q hello a.txt \
  && tar --zstd -cf r.tzst a.txt && rm a.txt && tar --zstd -xf r.tzst && grep -q hello a.txt \
  && file r.zip | grep -qi zip && echo ROUNDTRIP-OK'
```

Expect: `ROUNDTRIP-OK`.

## S3 — One rebuild on upgrade, reuse after (US3; SC-003)

Build the pre-feature binary from the parent commit (`git worktree add <tmp> HEAD~1 && go build` there, or the previous tag). In a scratch dir with a fixed config (pin `claude.version` to avoid a latest-resolution race):

1. `kekkai-old up` → builds tag A (or reuses if present).
2. `kekkai-test up` → prints `building image <tag B>` with B ≠ A (one-time rebuild).
3. `kekkai-test up` again → no "building image" line (reuse of B).
4. Config with `apt_packages: [ripgrep]` → `kekkai-test up` builds and the sandbox works; `rg` runs (duplicate is harmless).

## S4 — Firewall and runtime unchanged (SC-004)

During the S1 live sandbox: startup shows the standard firewall lines and `probe OK: https://example.com blocked`. `kekkai-test exec curl -sI --max-time 5 https://deb.debian.org` fails (mirror not reachable at runtime). Never add example.com to allowed_domains.

## S5 — Image growth bound (SC-005)

`docker image inspect <old tag> <new tag> --format '{{.Size}}'` (old from S3 step 1, new from step 2): difference < 50 MB.

## S6 — Docs (FR-006)

SPECIFICATION.md §5.1 lists the nine packages (grep `ripgrep`); §6.3 exec-path bullet mentions the `fd` symlink. `go vet ./...` and `go build ./cmd/kekkai` clean.
