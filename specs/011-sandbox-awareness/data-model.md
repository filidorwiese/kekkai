# Data Model: Sandbox-awareness context

No persistent data. Everything is computed at `kekkai up` and lives in the
container's process environment/args.

## Entities

### Sandbox-awareness prompt

| Part | Source | Rules |
|---|---|---|
| Pinned text | Go constant (FR-007) | verbatim from spec "Prompt text" section; never templated |
| Config summary (FR-008) | parsed `.kekkai.yaml` | appended below pinned text; ≤10 lines; per list max 8 items then `...and N more`; omitted when empty; `allow_all` → `network: unrestricted (allow_all)` |

Delivered via `KEKKAI_SYSTEM_PROMPT` env var → quoted `--append-system-prompt`
in the image CMD. Byte-for-byte integrity is the contract.

### Sandbox marker

`KEKKAI_SANDBOX=1` — builtin env (§5.3), set unconditionally (independent of the
version gate), user-overridable like all builtins.

### Version gate

| Input | Value |
|---|---|
| Resolved claude version | from `ensureImage` (pinned, or `latest` resolution); unknown on registry-fallback image reuse |
| Threshold | code constant: first Claude Code version with interactive `--append-system-prompt` (research.md R2) |

| Resolved version | Action |
|---|---|
| ≥ threshold | set `KEKKAI_SYSTEM_PROMPT` |
| < threshold | skip + one yellow warning |
| unknown | skip + one yellow warning (conservative: an unrecognized flag must never break startup) |

## State transitions

None — values are computed once per `up` and immutable for the container's
lifetime. A config change alters the summary only after `kekkai up` restart
(consistent with every other runtime input).
