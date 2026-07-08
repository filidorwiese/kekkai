# Implementation Plan: Disable Nonessential Traffic

**Branch**: `007-disable-nonessential-traffic` | **Date**: 2026-07-08 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/007-disable-nonessential-traffic/spec.md`

## Summary

Every sandbox gets `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` as a builtin
env var (user `env` can override via docker's last-value-wins), and
`statsig.anthropic.com` leaves the firewall's builtin allowlist — the builtin
external destination set shrinks to `api.anthropic.com`. All statsig mentions
disappear from README, SPECIFICATION.md, and the `kekkai init` starter
template. Two code sites (`internal/runtime/up.go` builtin env block,
`embed/init-firewall.sh` builtin hosts) plus docs; the firewall script is a
bake-time hash input, so existing images rebuild automatically on next `up`.

## Technical Context

**Language/Version**: Go (stdlib), existing toolchain; bash firewall script

**Primary Dependencies**: none new

**Storage**: N/A (kekkai stays stateless)

**Testing**: End-to-end per constitution IV — quickstart scenarios against the real binary and docker daemon

**Target Platform**: linux + darwin/arm64 (unchanged)

**Project Type**: CLI

**Performance Goals**: No startup change beyond the one-time image rebuild triggered by the firewall-script hash change

**Constraints**: Firewall verification probes must keep passing; env ordering (builtin → user → firewall) unchanged; no new config surface

**Scale/Scope**: ~5 lines of code across 2 files + 2 docs + starter template comment

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Spec-First**: SPECIFICATION.md §5.3 (builtin env), §5.4 (builtin
  destinations), §9.4 (firewall step) amended in the same commit as the code.
  PASS
- **II. Container is the security boundary**: The builtin destination set
  *shrinks* — a §5.4 change, made via the required spec amendment. The
  firewall script is edited to remove an allowance, never to relax one; the
  sudoers grant, verification probes, and lockdown order are untouched. PASS
- **III. Minimal Surface**: No new dependencies, flags, config keys, or
  exports; one builtin env constant and one deleted allowlist line. PASS
- **IV. End-to-End Validation**: quickstart scenarios run the real binary,
  inspect the real container env, and exercise the real firewall probes. PASS

Post-design re-check: unchanged, PASS.

## Project Structure

### Documentation (this feature)

```text
specs/007-disable-nonessential-traffic/
├── plan.md              # This file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   └── nonessential-traffic-cli.md
└── tasks.md             # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
internal/runtime/up.go        # buildRunArgs: builtin env block gains
                              # CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
embed/init-firewall.sh        # drop `add_domain statsig.anthropic.com warn`
                              # + comment update (hash input → auto rebuild)
internal/runtime/init.go      # starter template network comment loses statsig
README.md                     # network example comment loses statsig
SPECIFICATION.md              # §5.3 builtin env list, §5.4 destinations,
                              # §9.4 ipset build step
```

**Structure Decision**: Existing layout; no new files or packages. The env
var joins the existing builtin env block in `buildRunArgs` (before user env,
so user `env` overrides by docker last-value-wins — §7.3 ordering is already
the FR-002 precedence mechanism).

## Complexity Tracking

No constitution violations; table not needed.
