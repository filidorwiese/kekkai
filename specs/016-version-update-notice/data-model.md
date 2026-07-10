# Data Model: Update Notice on `kekkai version`

**Feature**: 016-version-update-notice | **Date**: 2026-07-10

No new data; two existing entities gain a surface, one helper changes visibility.

## Entities

### 1. Update notice (existing, specs/005)

| Aspect | Value |
|---|---|
| Producer | `selfupdate.Notice(version)` — unchanged |
| Text | `A new version of kekkai is available (<installed> -> <latest>), run 'kekkai self-update' to upgrade` (pinned, specs/005 contract) |
| Emptiness conditions | dev build, current, ahead, any lookup error/timeout — unchanged |
| Surfaces | `kekkai up` (stdout, pre-handoff, async) — unchanged; **new**: `kekkai version` (stderr, synchronous, after the version line) |

### 2. `kekkai version` output contract (changed)

| Stream | Content |
|---|---|
| stdout | version string, one line, always, first — byte-identical to pre-feature |
| stderr | nothing, or exactly one advisory line (notice text, yellow per convention) |
| exit code | 0 always (unchanged) |

### 3. Yellow advisory helper (visibility change)

| Aspect | Before | After |
|---|---|---|
| Name | `yellow` (unexported, `internal/runtime/up.go`) | `Yellow` (exported, same file) |
| Behavior | ANSI yellow iff target is a terminal and `NO_COLOR` unset | unchanged |
| Callers | up: update notice (stdout), missing-config warning (stderr), sandbox-context warning (stderr) | + `main.go` version advisory (stderr) |

**Relationships**: `main.go version case → selfupdate.Notice → (non-empty) → runtime.Yellow(os.Stderr)`. No state, no persistence.
