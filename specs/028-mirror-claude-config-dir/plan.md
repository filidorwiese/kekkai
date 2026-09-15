# Implementation Plan: Mirror Claude Config Dir at Host Path

**Branch**: `028-mirror-claude-config-dir` | **Date**: 2026-09-14 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/028-mirror-claude-config-dir/spec.md`

## Summary

Claude Code stores host-absolute paths under its config dir (plugin and
marketplace registries, hook/statusline/MCP commands). The sandbox mounts that
dir at `/home/kekkai/.claude`, so every stored path dangles and plugins silently
fail to load. Fix: resolve the host config dir (`$CLAUDE_CONFIG_DIR`, else
`~/.claude`), bind it rw at the identical path in the container, point the
container's `CLAUDE_CONFIG_DIR` at it, probe the same source on darwin, refuse
disk mounts that shadow it, and symlink `/home/kekkai/.claude` to it at container
start. Same pattern as specs/026 for `$PWD`. Host state untouched.

## Technical Context

**Language/Version**: Go 1.25, stdlib only (`flag`, `os`, `path/filepath`)

**Primary Dependencies**: docker CLI on host; `embed/Dockerfile.tmpl` (POSIX sh CMD)

**Storage**: none new. Host `~/.claude` (or `$CLAUDE_CONFIG_DIR`) bind-mounted rw

**Testing**: end-to-end against a real docker daemon (constitution IV); no unit-test suite exists in the repo. Validation via `quickstart.md`

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 host; debian:trixie container

**Project Type**: CLI, single static binary

**Performance Goals**: zero added docker calls on the happy path; one extra `ln -s` at container start

**Constraints**: config dir path must not enter the image (§6.1); container home stays `/home/kekkai`; no host file rewritten; `--rm` containers, no migration

**Scale/Scope**: ~5 Go touch points (`identity.go`, `up.go`, `preflight_darwin.go`, `validate.go` or `up.go` for the target check), 1 template line, 1 prompt constant, SPECIFICATION/README/demo wording

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Spec-First | PASS | SPECIFICATION.md §5.2, §5.3, §7.1, §7.3, §7.4 amended in the same commit as code; README digest updated |
| II. Container Is the Boundary | PASS | Same directory, same rw semantics as today; only its container path moves. No new sudo, no socket, firewall untouched. Not in §11 out-of-scope list |
| III. Minimal Surface | PASS | No new subcommand, no new config key, no dependency. One helper `ClaudeConfigDir()` in `identity.go`; `ValidateProjectPath` generalized rather than duplicated. Darwin preflight stays capability-based |
| IV. End-to-End Validation | PASS | quickstart.md defines real-daemon checks: plugin list parity, hook by absolute path, relocated config dir, symlink, image-tag parity across users |
| Constraint: image hash bake-time only | PASS | Path is a runtime `-v`/`-e`; CMD gains a path-free `ln -s "$CLAUDE_CONFIG_DIR"` step. One release rebuild, no per-user variance |

Post-design re-check: unchanged, PASS.

## Project Structure

### Documentation (this feature)

```text
specs/028-mirror-claude-config-dir/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── sandbox-layout.md
└── tasks.md              # /speckit-tasks output, not created here
```

### Source Code (repository root)

```text
internal/runtime/
├── identity.go           # + ClaudeConfigDir(); ValidateProjectPath → validateMirrorPath(kind, p)
├── up.go                 # Up: validate config dir + disk-mount targets; buildRunArgs: -v/-e use it
├── preflight_darwin.go   # probe binds resolved config dir source
└── sandboxprompt.go      # filesystem line mentions the config dir
internal/config/
└── validate.go           # unchanged unless target check lands here (research R3: lands in up.go)
embed/
└── Dockerfile.tmpl       # mkdir line drops /home/kekkai/.claude; CMD prepends symlink step
SPECIFICATION.md          # §5.2 §5.3 §7.1 §7.3 §7.4
README.md                 # lines ~35, ~195, ~203
docs/my-project/demo-sandbox.txt
```

**Structure Decision**: existing single-binary layout; no new packages or files besides spec artifacts.

## Design

### Resolution (identity.go)

`ClaudeConfigDir()`: if `$CLAUDE_CONFIG_DIR` non-empty, `filepath.Abs(filepath.Clean(v))`;
else `filepath.Join(os.UserHomeDir(), ".claude")`. No `EvalSymlinks` (research R1).
Single source for `Up`, `buildRunArgs`, `preflight`.

### Validation (up.go, one-pass report)

- `validateMirrorPath("claude config dir", dir)`: same three checks as the project
  path (root, ancestor of a protected container path, `:`/control chars). Exposed as
  `ValidateProjectPath` and `ValidateClaudeConfigDir` thin wrappers, or one
  function with a label argument.
- Disk-mount target check: for each non-skipped `cfg.Disk.Mounts[i]`, error if
  `ContainerPath == dir` or is a descendant. Lives in `Up` next to
  `ValidateProjectPath` (needs the resolved dir; config package stays unaware of
  runtime identity).
- `protectedContainerPaths` NOT extended (research R3).

### Run args (buildRunArgs)

- `os.MkdirAll(dir, 0o700)` as today, then `-v dir:dir`.
- `addEnv("CLAUDE_CONFIG_DIR", dir)` in the builtin block (user env may override, as today).
- Bind order unchanged: builtin mounts before disk mounts; docker sorts nested destinations by depth, so config-dir-inside-project and project-inside-config-dir both resolve (research R2).

### Preflight (darwin)

Replace `filepath.Join(home, ".claude")` source with `ClaudeConfigDir()`; probe
destination `/kekkai-probe/claude:ro` unchanged.

### Container start (Dockerfile.tmpl)

- Drop `/home/kekkai/.claude` from the `mkdir -p` line (an existing directory blocks `ln -s`).
- CMD prefix, before the startup echo:
  `if [ "$CLAUDE_CONFIG_DIR" != /home/kekkai/.claude ]; then ln -sfn "$CLAUDE_CONFIG_DIR" /home/kekkai/.claude; fi &&`
  Path-free: the template renders identically for every user. Runs as `kekkai`,
  home is user-owned, no sudo.
- `kekkai shell`/`exec` attach after CMD ran, so the link exists for them too.

### Prompt (sandboxprompt.go)

Filesystem bullet gains: "your Claude config directory (also at its host path)".
Exact text in `contracts/sandbox-layout.md`.

### Docs

SPECIFICATION §5.2 bullet, §5.3 env value, §7.1 helper, §7.3 validation order,
§7.4 probe list; README mount paragraph, security note, refusal note; demo text
comment after `ls -ld ~/.claude` ("a symlink to the host path").

## Complexity Tracking

No constitution violations. Empty.
