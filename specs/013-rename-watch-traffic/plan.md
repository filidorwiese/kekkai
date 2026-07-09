# Implementation Plan: Rename `kekkai watch` to `kekkai traffic`

**Branch**: `013-rename-watch-traffic` | **Date**: 2026-07-09 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/013-rename-watch-traffic/spec.md`

## Summary

Pure rename of the feature 010 egress-inspection command: `kekkai watch` becomes `kekkai traffic` with zero behavior change. Touches the dispatch case, the runtime file/function name, every tool-produced string that names the command (usage, help, stale-image hint), and all living docs (SPECIFICATION.md §3/§5.1/§9, README, CLI contract). `watch` becomes a plain unknown command. Firewall script comments naming the command are updated too — a comment-only edit that costs a one-time cheap image rebuild (spec assumption amended, research R2).

## Technical Context

**Language/Version**: Go 1.25 (module `kekkai`), bash (firewall script comments only)

**Primary Dependencies**: None added or removed

**Storage**: None

**Testing**: End-to-end against a real docker daemon (constitution IV); scenarios in `quickstart.md` — a subset of feature 010's, driven via the new name

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (unchanged)

**Project Type**: CLI (single static binary) + embedded firewall script

**Performance Goals**: N/A — no behavior change (FR-001)

**Constraints**: Zero behavior change except the strings that name the command (usage error, help text, stale-image hint). No firewall rule, verdict, or container changes (FR-005). No alias for `watch` (spec assumption: hard rename, pre-release)

**Scale/Scope**: 1 file rename + symbol rename, 1 dispatch case, ~6 doc locations, firewall script comments, new 013 CLI contract superseding 010's

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | SPECIFICATION.md §3 (command entry + bullet + contract pointer), §5.1 (tcpdump note), §9 (NFLOG paragraph) renamed in the same change as code; README digest follows | PASS (planned) |
| II. Container Is the Security Boundary | `init-firewall.sh` edits are comment text only — no rule, policy, ACCEPT/REJECT, probe, or sudoers change; diff reviewed to be comment-only (research R2) | PASS |
| III. Minimal Surface | No new subcommand, package, dependency, or config key — same command, new name, no alias. Naming departs from verb style; user decision recorded in spec Clarifications, supersedes 010 research R7 | PASS |
| IV. End-to-End Validation | quickstart.md re-runs the load-bearing 010 scenarios under the new name against a real sandbox; §9.6 probes re-asserted on the fresh `up` the rebuild triggers | PASS |

Post-design re-check: PASS — design introduced nothing beyond string/symbol renames and doc pointers.

## Project Structure

### Documentation (this feature)

```text
specs/013-rename-watch-traffic/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (no entities — stub)
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── traffic-cli.md   # Living CLI contract, supersedes 010's watch-cli.md
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/kekkai/
└── main.go              # case "watch" → "traffic"; usage help lines; usage-error string

internal/runtime/
├── traffic.go           # RENAMED from watch.go; Watch() → Traffic(); stale-image
│                        #   hint says 'kekkai traffic'; comments follow
└── up.go                # tcpdump package comment note: watch → traffic

embed/
└── init-firewall.sh     # comment-only: "kekkai watch" → "kekkai traffic" (4 sites)

SPECIFICATION.md         # §3 command line + behavior bullet + contract pointer,
                         #   §5.1 tcpdump note, §9 NFLOG paragraph
README.md                # command digest line

specs/010-inspect-egress/contracts/watch-cli.md
                         # header note: superseded by specs/013-.../traffic-cli.md
```

**Structure Decision**: §3 convention holds — dispatch in `cmd/kekkai/main.go`, logic in `internal/runtime/<name>.go`, so the runtime file follows the command name (`traffic.go`).

## Design Outline

1. **Command surface** (contracts/traffic-cli.md): identical to feature 010's contract with exactly three string substitutions — invocation `kekkai traffic`, usage error `usage: kekkai traffic (no arguments)`, stale-image hint `sandbox image predates 'kekkai traffic'; ...`. Banner, stream line formats, exit codes, all other strings byte-identical (FR-001). `watch` falls through to the standard unknown-command error (FR-002) — no code needed beyond removing the case.
2. **Go rename** (research R4): `internal/runtime/watch.go` → `traffic.go`, exported `Watch()` → `Traffic()`; internal identifiers/comments follow where they name the command.
3. **Docs** (FR-004): SPECIFICATION.md names `traffic` everywhere the command appears and points at the 013 contract for exact strings; 010's contract gets a one-line supersession note (history preserved, no duplicate living contract).
4. **Firewall script comments** (research R2): 4 comment lines say `kekkai watch`; renamed for accuracy. Comment-only diff, but the script is a bake-time input (§6.1) so image hash changes → one-time rebuild on next `up`. Spec assumption amended to record this.

## Complexity Tracking

No constitution violations — table not needed.
