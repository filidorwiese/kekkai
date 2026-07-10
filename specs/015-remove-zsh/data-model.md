# Data Model: Single-Shell Sandbox (bash) and Release Build Repair

**Feature**: 015-remove-zsh | **Date**: 2026-07-10

No new persistent data; the entities are existing constants, files, and CI configuration whose values change.

## Entities

### 1. Builtin apt package set (`internal/runtime/up.go`)

| Aspect | Before | After |
|---|---|---|
| Subcommand shells | `zsh` (kekkai shell) | — (bash, already present for nvm, serves kekkai shell) |
| Everything else | unchanged | unchanged |

Bake-time input → removal changes every image hash → rebuild on next `up`.

### 2. Sandbox user shell surfaces (`embed/Dockerfile.tmpl`)

| Surface | Before | After |
|---|---|---|
| `useradd -s` | `/bin/zsh` | `/bin/bash` |
| `ENV SHELL` | `/bin/zsh` | `/bin/bash` |
| rc file | append to `~/.zshrc` | append to `~/.bashrc` (Debian skel file exists via `useradd -m`) |

### 3. History persistence (per-project volume, unchanged identity)

| Aspect | Before | After |
|---|---|---|
| Volume name / mount | `kekkai-history-<sha256(pwd)[:8]>` at `/commandhistory` | same |
| File | `.zsh_history` | `.bash_history` (touched at build; created on demand in old volumes) |
| Incremental append | `setopt INC_APPEND_HISTORY` | `PROMPT_COMMAND='history -a'` |
| Size caps | `HISTSIZE=10000`, `SAVEHIST=10000` | `HISTSIZE=10000`, `HISTFILESIZE=10000` |
| Old volumes | — | `.zsh_history` orphaned in place; no migration (accepted reset) |

### 4. Builtin env set (`internal/runtime/up.go`, spec §5.3)

| Var | Before | After |
|---|---|---|
| `POWERLEVEL9K_DISABLE_GITSTATUS=true` | injected | removed (not reserved; user `env` may set it) |
| All others | unchanged | unchanged |

### 5. Release toolchain declaration (`.github/workflows/release.yml`)

| Aspect | Before | After |
|---|---|---|
| setup-go input | `go-version: "1.24"` (drifted from go.mod) | `go-version-file: go.mod` (single source of truth) |
| `GOTOOLCHAIN` | `local` (setup-go@v6 default) | same — now always satisfied |

### 6. `kekkai shell` command (`internal/runtime/shell.go`, `cmd/kekkai/main.go`)

| Aspect | Before | After |
|---|---|---|
| exec target | `docker exec -it <ctr> zsh` | `docker exec -it <ctr> bash` |
| help text | "open zsh in the running sandbox" | "open bash in the running sandbox" |

**Relationships**: entities 1–3 are bake-time (image hash changes once, together); 4 is runtime env assembly; 5 is CI-only; 6 is host-side CLI. No interdependencies beyond landing in one release.
