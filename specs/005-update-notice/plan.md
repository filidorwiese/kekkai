# Implementation Plan: Update Notice on Up

**Branch**: `005-update-notice` | **Date**: 2026-07-07 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/005-update-notice/spec.md`

## Summary

`kekkai up` starts a background check against the same GitHub latest-release endpoint
`self-update` uses (same repo slug, same `KEKKAI_REPO` override, same version
comparison). Just before the interactive handoff, if the check has finished and found a
newer release, one friendly line names the version and `kekkai self-update`; in every
other case (current, ahead, dev build, any failure, result not ready) nothing is
printed. The check is never awaited — startup latency is unchanged by construction.

## Technical Context

**Language/Version**: Go 1.24, stdlib only (net/http, goroutine + buffered channel)

**Primary Dependencies**: existing `internal/selfupdate` internals (`latestTag`, `compareVersions`, `repoSlug`) — new exported `Notice(version) string` in the same package, so nothing else is exported

**Storage**: N/A — no state, no throttle file (spec assumption)

**Testing**: End-to-end per constitution IV: build with `-ldflags -X main.version=<v>` variants, observe `up` output

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (pure Go, no platform code)

**Project Type**: CLI tool

**Performance Goals**: Zero added startup latency — check runs concurrently with validation/image work; non-blocking read at print point (skip if not ready)

**Constraints**: Silent on every non-"newer" outcome (FR-003/004); notice only on the success path before handoff, never amid error output; GitHub call capped by the existing 10s client timeout

**Scale/Scope**: 2 Go files touched (`internal/selfupdate/selfupdate.go`, `internal/runtime/up.go`), SPECIFICATION.md, ~40 lines total

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Assessment | Status |
|-----------|-----------|--------|
| I. Spec-First | SPECIFICATION.md gains the notice behavior (§3 `up`, §10 self-update cross-ref) in the same commit | PASS |
| II. Container Is the Security Boundary | Host-side HTTPS to api.github.com (endpoint self-update already calls); nothing crosses into the sandbox | PASS |
| III. Minimal Surface | No new package, no config key, no state file; one exported function + ~10 lines in up.go | PASS |
| IV. End-to-End Validation | quickstart: ldflags version variants + `KEKKAI_REPO` against real GitHub, real `up` runs | PASS |

**Post-Phase-1 re-check**: PASS — design introduces no additional surface beyond the above.

## Project Structure

### Documentation (this feature)

```text
specs/005-update-notice/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── update-notice-cli.md   # exact output contract
└── tasks.md             # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
internal/selfupdate/selfupdate.go   # + Notice(version string) string
internal/runtime/up.go              # goroutine at Up() start; non-blocking
                                    #   print just before docker.Interactive
SPECIFICATION.md                    # §3 up behavior + §10 note
```

**Structure Decision**: The check logic lives in `internal/selfupdate` next to the
internals it reuses (release endpoint, comparison, repo override); `up.go` only owns
concurrency and placement. Runtime calls one exported function — §3's "self-update
lives outside runtime, no docker involvement" boundary stays intact.

## Complexity Tracking

No constitution violations — table not needed.
