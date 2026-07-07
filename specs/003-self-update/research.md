# Research: Self-Update Command

## R1: Latest release discovery

- **Decision**: `GET https://api.github.com/repos/${REPO}/releases/latest` with net/http (10s timeout), parse `tag_name` via encoding/json. `REPO` defaults to `filidorwiese/kekkai`, overridable with `KEKKAI_REPO` (install.sh precedent, enables e2e testing against a fork).
- **Rationale**: identical source of truth as install.sh; unauthenticated rate limit (60/h/IP) is plenty for a manual command. HTTP 403/429 mapped to a "rate limited, retry later" error per spec edge case.
- **Alternatives considered**: scraping the releases page (fragile); a version manifest file in the repo (second source of truth, violates the one-channel assumption); go-github library (violates Minimal Surface).

## R2: Version comparison

- **Decision**: parse `vMAJOR.MINOR.PATCH` numerically (tiny hand-rolled compare, handles a pre-release suffix by comparing the numeric core). installed == latest or installed > latest → up-to-date outcome, exit 0, zero writes. installed < latest → update.
- **Rationale**: spec edge case demands "newer than latest = up to date"; string equality alone cannot express that. Numeric compare is ~20 LOC, no dependency.
- **Alternatives considered**: golang.org/x/mod/semver (extra dependency for 20 LOC); string equality only (wrong on ahead-of-release builds).

## R3: Artifact download + integrity

- **Decision**: download `kekkai_${TAG}_${GOOS}_${GOARCH}.tar.gz` and `SHA256SUMS` from `https://github.com/${REPO}/releases/download/${TAG}/`; compute sha256 (crypto/sha256) of the tarball, compare against the manifest line before extracting `kekkai` from the tar (archive/tar + compress/gzip). Missing artifact (404) → distinct "no <os>/<arch> artifact in release <tag>" error.
- **Rationale**: byte-for-byte the install.sh contract (contracts/distribution.md of feature 002); checksum-before-extract keeps a tampered tarball from ever being unpacked.
- **Alternatives considered**: signature verification (no signing infra exists; spec assumption pins checksum as the bar); downloading the raw binary (releases only ship tarballs).

## R4: Atomic in-place replacement

- **Decision**: resolve the real binary path via `os.Executable()` + `filepath.EvalSymlinks`. Preflight before any download: target directory and file writable (open-for-write probe). Extract to `<dir>/.kekkai-update-<pid>` (same filesystem), chmod 0755, `os.Rename` onto the target. Temp file removed on any failure path.
- **Rationale**: rename(2) within one filesystem is atomic on Linux and macOS; the running process keeps its inode so self-replacement is safe. Writing the temp file next to the target (not in $TMPDIR) guarantees same-filesystem rename. Preflight satisfies FR-008 (fail before download).
- **Alternatives considered**: write + copy over (not atomic, can truncate on interrupt: violates FR-004); re-exec install.sh (needs curl-pipe trust and loses from/to reporting); privilege escalation for root-owned paths (explicitly out of scope per spec assumption).

## R5: Development build guard

- **Decision**: `version == "dev"` (the un-ldflags default in cmd/kekkai/main.go) or any version not starting with `v` refuses with a pointer to the curl-pipe installer.
- **Rationale**: FR-007; a dev build has no corresponding release artifact and overwriting a `go build` output silently would surprise.
- **Alternatives considered**: allowing dev builds to force-update with a flag (surface growth, no demonstrated need).

## R6: Package placement

- **Decision**: new package `internal/selfupdate`, one file, called from `cmd/kekkai/main.go` as `selfupdate.Run(version)`.
- **Rationale**: `internal/runtime` is the docker/sandbox layer; self-update touches neither docker nor config. Separate package keeps both readable.
- **Alternatives considered**: `internal/runtime/selfupdate.go` (muddles the §3 "logic in internal/runtime" convention, but that convention describes sandbox subcommands; SPECIFICATION §3 amendment will note the exception).

## R7: What deliberately stays out

- No `--check` / `--version X` / downgrade flags: forward-only per spec assumption; every flag is surface.
- No update notification on other commands (nag on `up`): out of scope, would add a network call to the critical path.
- No Windows handling: platform out of scope per constitution.
