# Implementation Plan: Sandbox-awareness context for Claude Code

**Branch**: `011-sandbox-awareness` | **Date**: 2026-07-09 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/011-sandbox-awareness/spec.md`

## Summary

At sandbox start, kekkai appends a pinned sandbox-awareness prompt to Claude Code's system prompt and sets `KEKKAI_SANDBOX=1`. The prompt cannot ride the existing `CLAUDE_ARGS` env (its CMD expansion is deliberately unquoted → word-splitting would shred multiline text); instead it travels in its own env var `KEKKAI_SYSTEM_PROMPT`, and the image CMD gains a quoted conditional: flag added only when the var is non-empty. Host side, `up` sets the var only when the resolved Claude version supports `--append-system-prompt` interactively (threshold constant, compared with the existing selfupdate semver helper); unsupported or unknown → one yellow warning, var unset, startup proceeds. Prompt text lives as a Go constant; the optional config summary (allowed domains/CIDRs, shadowed names) is rendered from the already-parsed config — straightforward, so FR-008 is in.

## Technical Context

**Language/Version**: Go 1.25 (module `kekkai`); Dockerfile template CMD line

**Primary Dependencies**: None new. Reuses `internal/selfupdate` semver compare (exported) and the parsed `config.Config` already in `buildRunArgs`

**Storage**: None — no files written anywhere (FR-004 forbids workspace/`~/.claude` writes; nothing else needed)

**Testing**: End-to-end against a real docker daemon (constitution IV); scenarios in `quickstart.md`; SC-001/SC-002 validated manually with one prompt each

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (no platform branches)

**Project Type**: CLI (single static binary) + embedded Dockerfile template

**Performance Goals**: Zero added startup latency (no network calls; version already resolved by `ensureImage`)

**Constraints**: Prompt must survive delivery byte-for-byte (quoting); injection must never break startup (FR-006); no rebuild triggered by runtime config (§6.1 — the CMD change is a bake-time input change shipped with the release, one rebuild per project on upgrade, allowed)

**Scale/Scope**: 1 new Go file (prompt constant + renderer + version gate), `up.go` env/version wiring, Dockerfile CMD line, selfupdate export, SPECIFICATION.md §5.3/§6.3/§7.2 + README

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | SPECIFICATION.md amended in the same change: §5.3 builtin env (`KEKKAI_SANDBOX`, `KEKKAI_SYSTEM_PROMPT`), §6.3 Dockerfile CMD contract, §7.2 startup line | PASS (planned, tasked) |
| II. Container Is the Security Boundary | Nothing crosses the boundary: two env vars in, no new mounts, no firewall change, no sudo change, no writes to shared `~/.claude` (FR-004 is itself a boundary-hygiene requirement). Prompt explicitly instructs the agent NOT to fight the sandbox — aligned with the threat model | PASS |
| III. Minimal Surface | No new subcommand, no config keys, no deps. One new builtin env pair (§5.3 list grows by two), one constant file. Version gate reuses existing semver code (exported, not duplicated). FR-008 templating uses the config already in hand — no new parsing | PASS |
| IV. End-to-End Validation | quickstart.md scenarios incl. §9.6 probe regression; prompt-delivery verified by inspecting the started container's env/args, model behavior spot-checked manually | PASS |

Post-design re-check: PASS — design added only the truncation rule for the rendered summary (display logic).

## Project Structure

### Documentation (this feature)

```text
specs/011-sandbox-awareness/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── sandbox-prompt.md # Prompt/env/CMD contract: exact strings, gating table
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/runtime/
├── sandboxprompt.go     # NEW: prompt constant, config-summary renderer, version gate
└── up.go                # ensureImage returns resolved version; buildRunArgs sets
                         # KEKKAI_SANDBOX=1 + KEKKAI_SYSTEM_PROMPT (gated), warning path

internal/selfupdate/
└── selfupdate.go        # compareVersions → CompareVersions (exported, unchanged logic)

embed/
└── Dockerfile.tmpl      # CMD: conditional quoted --append-system-prompt "$KEKKAI_SYSTEM_PROMPT"

SPECIFICATION.md         # §5.3 env, §6.3 CMD contract, §7.2 lifecycle line
README.md                # brief mention under What you get / limitations as fits
```

**Structure Decision**: Logic stays in `internal/runtime` per §3; the prompt is a Go constant (FR-007), not an embed file — it is code-reviewed text, not an asset with independent lifecycle.

## Design Outline

1. **Delivery path** (research.md R1): `KEKKAI_SYSTEM_PROMPT` env var + CMD
   `sudo /usr/local/bin/init-firewall.sh && if [ -n "$KEKKAI_SYSTEM_PROMPT" ]; then exec claude $CLAUDE_ARGS --append-system-prompt "$KEKKAI_SYSTEM_PROMPT"; else exec claude $CLAUDE_ARGS; fi`.
   Quoted expansion preserves newlines/quotes byte-for-byte; `$CLAUDE_ARGS` stays unquoted by design (it is a flag string, unchanged behavior). Empty/unset var → binary-identical launch to today (FR-006, US3). Ours is appended after user args, so a user's own `--append-system-prompt` in `claude.args` survives alongside (edge case).
2. **Version gate** (research.md R2): `ensureImage` returns the resolved claude version alongside the tag; registry-fallback image reuse returns version unknown. Gate: `version >= threshold` → set var; unknown or below → skip + one yellow stderr warning (`warnNoConfig` convention). Threshold is a named constant next to the prompt.
3. **Marker env** (research.md R3): `KEKKAI_SANDBOX=1` added to the §5.3 builtin block in `buildRunArgs`, before user env (user-overridable like every builtin — consistency over lockdown).
4. **Prompt + summary** (research.md R4): pinned text as `const sandboxPrompt`; FR-008 renderer appends "Current sandbox config:" with allowed domains+CIDRs and shadowed names from `cfg`, capped (first 8 items per list, `...and N more`), ≤10 lines total; omitted entirely when nothing to list. `allow_all: true` renders "network: unrestricted (allow_all)" instead of a domain list.
5. **No-writes guarantee** (FR-004): feature touches only process args/env — nothing writes to `/workspace` or `~/.claude`; asserted in quickstart by checksumming the workspace and `~/.claude` before/after.

## Complexity Tracking

No constitution violations — table not needed.
