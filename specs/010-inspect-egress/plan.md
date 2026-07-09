# Implementation Plan: Inspect egress traffic (`kekkai watch`)

**Branch**: `010-inspect-egress` | **Date**: 2026-07-09 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/010-inspect-egress/spec.md`

## Summary

New `watch` subcommand streams a live, labeled log of the sandbox's egress: connection attempts with ALLOW/BLOCK verdicts plus DNS queries/answers, so developers can discover which hostnames to add to `network.allowed_domains`. Mechanism: `init-firewall.sh` gains passive NFLOG rules (group 1 = allowed + DNS, group 2 = blocked) that observe packets without changing any verdict; `kekkai watch` attaches two in-container `tcpdump -i nflog:<group>` readers via `docker exec -u root`, merges and formats their streams on the host, and annotates connection lines with hostnames learned from DNS answers. Always-on plumbing (rules + one apt package baked in) satisfies attach-anytime; under `allow_all` the observe-only rules are installed with policy ACCEPT so watch still works.

## Technical Context

**Language/Version**: Go 1.25 (module `kekkai`), bash (firewall script)

**Primary Dependencies**: No new Go deps. New builtin apt package: `tcpdump` (§5.1) — the nflog reader that needs zero extra config. Docker CLI on host.

**Storage**: None persisted — in-memory IP→hostname cache for the watch session only

**Testing**: End-to-end against a real docker daemon (constitution IV); scenarios in `quickstart.md`

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64. NFLOG needs the runtime VM kernel's `nfnetlink_log`; on macOS this is capability-probed at watch start (clear error if absent), community-validated tier (constitution III/IV)

**Project Type**: CLI (single static binary) + embedded firewall script

**Performance Goals**: Only NEW connections and DNS packets are logged (no per-packet flood); first event visible within ~1s of traffic

**Constraints**: NFLOG rules MUST be observe-only (FR-005): no policy, ACCEPT, or REJECT line changes; verification probes untouched. Watch never creates/restarts/removes containers.

**Scale/Scope**: 1 new subcommand, 1 new Go file, firewall script + builtin package list touched, SPECIFICATION.md §3/§5.1/§9 amendments

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | SPECIFICATION.md §3 (watch entry), §5.1 (tcpdump), §9 (NFLOG log rules incl. allow_all observe-only block) amended in the same change as code | PASS (planned, tasked) |
| II. Container Is the Security Boundary | `init-firewall.sh` changes are strictly additive NFLOG (observe-only) rules: default DROP policies, the ipset ACCEPT, the REJECT line, and all verification probes are untouched — no destination opens, no verdict changes. Sudoers unchanged (readers run via host-side `docker exec -u root`, which the host user can already do). §5.4 destination set unchanged. Watch reads packets; it cannot alter the firewall (FR-005) | PASS — flagged for extra review scrutiny per constitution governance |
| III. Minimal Surface | New subcommand justified: discovering required endpoints is currently blind trial-and-error; spec'd user need. One new apt package (`tcpdump`, needed as the nflog reader), zero Go deps, zero config keys, no flags. macOS support stays capability-probed (error message may name the runtime), no per-runtime code | PASS |
| IV. End-to-End Validation | quickstart.md scenarios run against a real sandbox; firewall verification probes remain part of startup and are re-asserted in regression scenarios | PASS |

Post-design re-check: PASS — design added only the in-memory hostname cache and a repeat-suppression window, both host-side display logic.

## Project Structure

### Documentation (this feature)

```text
specs/010-inspect-egress/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── watch-cli.md     # CLI contract: line formats, exit codes, error strings
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/kekkai/
└── main.go              # + case "watch" (no flags; extra args → usage error)

internal/runtime/
├── watch.go             # NEW: resolve container, spawn readers, merge/format/annotate
└── up.go                # builtinAptPackages += "tcpdump" (subcommands tier)

embed/
└── init-firewall.sh     # + NFLOG rules (groups 1/2); + observe-only block in allow_all path

SPECIFICATION.md         # §3 watch entry, §5.1 tcpdump, §9 log-rule + allow_all amendments
README.md                # user-facing digest: watch in command list
```

**Structure Decision**: §3 convention — dispatch in `cmd/kekkai/main.go`, logic in `internal/runtime/watch.go`. Firewall edits stay inside the sanctioned script.

## Design Outline

1. **Firewall rules** (research.md R1, R6): group 1 (allowed): DNS queries (`udp dport 53`, positioned ahead of the loopback/DNS ACCEPTs so embedded-DNS traffic is seen), DNS answers (`udp sport 53` on INPUT, feeds the IP→hostname map), NEW connections matching the ipset or bridge subnet. Group 2 (blocked): `--state NEW` immediately before the REJECT. `allow_all` path: same group-1 style rules with policy ACCEPT, no group 2, installed before the early `exit 0`.
2. **Readers** (research.md R2): `docker exec -u root <id> tcpdump -l -n -tt -i nflog:1` and `nflog:2`. tcpdump is the §5.1 addition; image hash changes (§6.1) so existing projects rebuild on next `up` — expected and cheap (late apt layer... no: apt is an early layer, full rebuild — accepted, one-time).
3. **Merge/format** (research.md R3, contract): two goroutines scan reader stdout into one channel; lines parsed to `HH:MM:SS ALLOW|BLOCK <proto> <ip>:<port> (<hostname?>)` and `HH:MM:SS DNS query|answer ...`; unparseable lines pass through raw (never dropped). Hostname annotation from the session's DNS-answer cache; repeat suppression per event tuple within a short window, first occurrence always shown.
4. **Lifecycle** (research.md R4): SIGINT/SIGTERM → kill reader processes, best-effort `docker exec -u root <id> pkill -x tcpdump` (docker exec doesn't forward signals — feature 009 lesson), exit 0. Readers exiting on their own (sandbox stopped) → stderr message, exit 1. No sandbox → `no running sandbox for <pwd>, run 'kekkai up'`, exit 1.
5. **Stale image detection** (research.md R5): reader exits immediately with 126/127 → error telling the user the sandbox predates watch: `kekkai down && kekkai up` to rebuild.

## Complexity Tracking

No constitution violations — table not needed. (Principle II scrutiny note recorded in the gate table.)
