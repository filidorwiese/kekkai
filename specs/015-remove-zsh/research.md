# Research: Single-Shell Sandbox (bash) and Release Build Repair

**Feature**: 015-remove-zsh | **Date**: 2026-07-10

Sources: `.github/workflows/release.yml`, `go.mod`, `embed/Dockerfile.tmpl`, `internal/runtime/{up,shell}.go`, `cmd/kekkai/main.go`, `SPECIFICATION.md`, setup-go and bash documentation.

## R1. Why releases broke, and the fix

- **Finding**: `release.yml` pins `go-version: "1.24"`; `go.mod` declares `go 1.25.0`; `actions/setup-go@v6` sets `GOTOOLCHAIN=local` by default, which forbids the auto-download escape hatch — so `go build` refuses outright. Broke when go.mod moved to 1.25.0 while the workflow stayed at 1.24 (two sources of truth drifted).
- **Decision**: Replace `go-version: "1.24"` with `go-version-file: go.mod` in the setup-go step.
- **Rationale**: The module file becomes the single toolchain declaration (FR-001); a future `go.mod` bump updates CI automatically. Keeps `GOTOOLCHAIN=local` semantics (fast, no surprise downloads) because the installed toolchain now always satisfies the module.
- **Alternatives considered**:
  - Bump to `go-version: "1.25"` — fixes today, reintroduces the exact drift that broke it.
  - Set `GOTOOLCHAIN=auto` — setup-go downloads one toolchain, go downloads another; slower and two moving parts.

## R2. bash equivalent of the zsh history persistence

- **Decision**: Append to `/home/kekkai/.bashrc`: `export HISTFILE=/commandhistory/.bash_history`, `HISTSIZE=10000`, `HISTFILESIZE=10000`, `PROMPT_COMMAND='history -a'`. Keep `touch /commandhistory/.bash_history` + chown in the same Dockerfile block as today.
- **Rationale**: `PROMPT_COMMAND='history -a'` appends each command to HISTFILE as it is entered — the bash equivalent of zsh's `INC_APPEND_HISTORY` (FR-005: a crashed session keeps earlier commands). `HISTFILESIZE` is bash's on-disk cap (zsh's `SAVEHIST`). `docker exec -it bash` starts an interactive non-login shell → reads `~/.bashrc` → wiring applies to exactly the `kekkai shell` path. Debian's skel `.bashrc` (present via `useradd -m`) sets its own HISTSIZE defaults earlier in the file; the appended block wins (last assignment).
- **Alternatives considered**:
  - `history -a` via bash `trap ... EXIT` — loses everything on crash; rejected by FR-005.
  - Login-shell wiring (`.bash_profile`) — `docker exec bash` is non-login; wrong file.

## R3. Old history volumes (migration behavior)

- **Decision**: No migration. Bash creates `/commandhistory/.bash_history` on first `history -a` in pre-existing volumes; the orphaned `.zsh_history` stays.
- **Rationale**: The volume directory is kekkai-owned (initialized from the image on first use), so file creation succeeds. Spec explicitly accepts the one-time history reset; migrating zsh→bash history format (zsh extended-history timestamps) is throwaway code for a convenience file.
- **Alternatives considered**: Copying `.zsh_history` → `.bash_history` at container start — zsh extended format (`: <ts>:<dur>;cmd`) pollutes bash history; conversion logic for a convenience feature violates Minimal Surface.

## R4. Shell surfaces to flip

- **Decision**: Four surfaces, all flipped to bash: `useradd -s /bin/bash` (login shell), `ENV SHELL=/bin/bash` (advertised shell — Claude Code and tooling read it), `internal/runtime/shell.go` `docker exec -it <ctr> zsh` → `bash`, `cmd/kekkai/main.go` help string.
- **Rationale**: FR-002/FR-003/FR-004. All four verified as the complete set via repo-wide grep for `zsh` — nothing else references it (init-firewall.sh, sandboxprompt.go clean).
- **Alternatives considered**: Keeping `SHELL=/bin/zsh` unset instead of flipped — Claude Code falls back inconsistently; explicit bash is deterministic.

## R5. POWERLEVEL9K_DISABLE_GITSTATUS removal

- **Decision**: Delete the `addEnv("POWERLEVEL9K_DISABLE_GITSTATUS", "true")` line in `up.go`; do not add it to reserved env keys.
- **Rationale**: The var configured the powerlevel10k zsh theme from Anthropic's devcontainer heritage; kekkai's image never installed p10k, so it was dead even before zsh removal (FR-006). Not reserving it: harmless if a user sets it, and the reserved list is for keys kekkai manages.
- **Alternatives considered**: None substantive.

## R6. apt package removal scope

- **Decision**: Remove exactly `zsh` from `builtinAptPackages`. `bash` stays (nvm dependency, pinned in 014).
- **Rationale**: `zsh-common` is a dependency of zsh, disappears with it (~20MB combined). No other builtin depends on zsh. §5.1's "Required — subcommands" entry for zsh moves to bash serving `kekkai shell`.
- **Alternatives considered**: None.

## R7. Validation approach for the CI fix (no tag push required)

- **Decision**: Three checks without publishing a release: (a) `go version` of the toolchain that `go-version-file: go.mod` resolves — assert `setup-go` semantics locally via `go mod edit -json | jq .Go` matching; (b) run the workflow's exact build matrix locally (`GOOS`/`GOARCH`/`CGO_ENABLED=0 go build -trimpath -ldflags ...`) under a ≥1.25 toolchain; (c) YAML review that no other pinned Go version remains in `.github/`.
- **Rationale**: Constitution IV's "real daemon" principle applied to CI: exercise the same commands the workflow runs. An actual tag push (SC-001 end-to-end) happens at the next real release; the mechanism is verified beforehand.
- **Alternatives considered**: Pushing a throwaway tag — publishes a junk release to the public repo; rejected.
