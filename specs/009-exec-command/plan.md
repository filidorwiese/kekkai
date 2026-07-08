# Implementation Plan: kekkai exec

**Branch**: `009-exec-command` | **Date**: 2026-07-08 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/009-exec-command/spec.md`

## Summary

Add an `exec` subcommand that runs a one-off command inside the running sandbox for `$PWD` and exits with that command's exit code. It is the non-interactive sibling of `kekkai shell`: same label-based container resolution (`kekkai.cwd`), same `docker.Interactive` plumbing (stdio attach, signal forwarding, exit-code passthrough). New logic is ~30 lines in `internal/runtime/exec.go` plus dispatch in `cmd/kekkai/main.go`; the only new decision is conditional TTY allocation (`-t` only when stdin is a terminal, so pipes work).

## Technical Context

**Language/Version**: Go 1.24, stdlib only (module `kekkai`)

**Primary Dependencies**: `gopkg.in/yaml.v3` (existing, untouched); `golang.org/x/term` (new, isatty for conditional TTY — see research.md R1); docker CLI on host

**Storage**: N/A — no config keys, no files, no labels added

**Testing**: End-to-end against a real docker daemon (constitution IV); scenarios in `quickstart.md`

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (same as binary)

**Project Type**: CLI (single static binary)

**Performance Goals**: kekkai adds no measurable overhead; runtime dominated by `docker exec` + the command itself

**Constraints**: No new host dependencies; no flag parsing of user command words (FR-004); sandbox must keep running after exec ends (FR-008)

**Scale/Scope**: 1 new subcommand, 2 files touched + 1 new file, SPECIFICATION.md §3 + README amendment

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | `SPECIFICATION.md` §3 gains the `exec` line in the same change; README digest updated | PASS (planned, tasked) |
| II. Container Is the Security Boundary | `exec` enters the existing container via `docker exec`; no socket exposure, no sudo, no firewall change, no new network destinations | PASS |
| III. Minimal Surface | New subcommand justified: only scriptable one-shot path into the sandbox today is `shell` (interactive only). No config keys, no flags of its own (args verbatim), reuses `docker.Interactive` and label resolution. One new dependency, `golang.org/x/term`: e2e showed a stat-based TTY check misfires on `/dev/null` stdin, a real isatty is required, and the stdlib alternative is build-tagged raw ioctls with `unsafe` on two platforms — more surface than the official one-function module | PASS |
| IV. End-to-End Validation | Validation is `kekkai up` + real `kekkai exec` runs per `quickstart.md`; no firewall probes skipped | PASS |

Post-design re-check: PASS — design added nothing beyond the above (one helper for TTY detection, reused from the §4.1 color-check pattern).

## Project Structure

### Documentation (this feature)

```text
specs/009-exec-command/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── exec-cli.md      # CLI contract: usage, streams, exit codes, exact strings
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/kekkai/
└── main.go              # + case "exec": strip optional leading --, require ≥1 word, dispatch

internal/runtime/
├── exec.go              # NEW: Exec(cmdArgs) (int, error) — mirror of shell.go
└── shell.go             # unchanged (reference implementation)

internal/docker/
└── exec.go              # unchanged: Interactive() already forwards signals + exit code

SPECIFICATION.md         # §3: exec line in CLI block + behavior bullet
README.md                # user-facing digest: exec in command list
```

**Structure Decision**: Follows the §3 convention verbatim — dispatch in `cmd/kekkai/main.go`, logic in `internal/runtime/exec.go`. No new packages.

## Design Outline

1. **Dispatch** (`cmd/kekkai/main.go`): `case "exec": code, err = runtime.Exec(execArgs(args[1:]))`. `execArgs` strips one optional leading `--`; empty result → `usage: kekkai exec [--] <command> [args...]` error, exit 1. No `flag.FlagSet` — every word belongs to the user's command (FR-004).
2. **Runtime** (`internal/runtime/exec.go`): copy `Shell()` shape — `os.Getwd()` → `docker.ContainersByLabel(LabelCwd+"="+pwd)` → first `Running` container → `docker.Interactive(append([]string{"exec", "-i", <"-t"?>, c.ID}, cmdArgs...)...)`. No running container → same error string family as shell: `no running sandbox for <pwd>, run 'kekkai up'`.
3. **TTY policy** (research.md R1): `-i` always; `-t` only when stdin is a terminal (`os.Stdin` `ModeCharDevice`, same check as `up.go`'s color gate). Without this, piped invocations die with docker's "the input device is not a TTY".
4. **Exit codes** (research.md R2): `docker.Interactive` already returns the child's exit code; `docker exec` passes through the command's code (and yields 125/126/127 for daemon/not-executable/not-found — spec edge case says those pass through as-is). Kekkai's own failures (no sandbox, usage) exit 1 via the existing dispatch error path.
5. **Signals** (research.md R3): existing `Interactive` forwards SIGINT/SIGTERM to the docker CLI child. With a TTY, ^C reaches the in-container process via the pty. Without a TTY, the docker CLI exits but the in-container process may linger (docker `exec` limitation) — recorded as a known limitation in the contract, sandbox keeps running either way.

## Complexity Tracking

No constitution violations — table not needed.
