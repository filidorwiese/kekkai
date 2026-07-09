# Contract: sandbox-awareness injection

## Container environment

| Var | Value | When |
|---|---|---|
| `KEKKAI_SANDBOX` | `1` | always (builtin §5.3, before user env, user-overridable) |
| `KEKKAI_SYSTEM_PROMPT` | pinned prompt text (+ optional config summary) | only when resolved claude version ≥ `1.0.51` |

## Image CMD (§6.3 / §7.2 amendment)

```
/bin/sh -c "sudo /usr/local/bin/init-firewall.sh && if [ -n \"$KEKKAI_SYSTEM_PROMPT\" ]; then exec claude $CLAUDE_ARGS --append-system-prompt \"$KEKKAI_SYSTEM_PROMPT\"; else exec claude $CLAUDE_ARGS; fi"
```

- `$KEKKAI_SYSTEM_PROMPT` expansions are double-quoted: the text arrives
  byte-for-byte (newlines, quotes, spaces intact).
- `$CLAUDE_ARGS` stays unquoted (unchanged: it is a flag string).
- Var empty/unset → the `else` branch is command-line-identical to the
  pre-feature CMD.
- kekkai's flag comes after `$CLAUDE_ARGS`: a user-supplied
  `--append-system-prompt` in `claude.args` remains a separate, earlier occurrence.
- Never `--system-prompt` / `--system-prompt-file` (replacement flags — forbidden, FR-001).

## Version gate

| Resolved claude version | Behavior |
|---|---|
| ≥ 1.0.51 | inject (set `KEKKAI_SYSTEM_PROMPT`) |
| < 1.0.51 | skip; one yellow stderr warning |
| unknown (registry-fallback image reuse) | skip; one yellow stderr warning |

Warning text (stderr, yellow per §4.1 convention, exactly once):
`warning: claude <version> does not support sandbox context injection (needs >= 1.0.51), starting without it`
— for the unknown case, `<version>` reads `version unknown` and the parenthetical is unchanged.
Threshold source: Claude Code CHANGELOG v1.0.51 — "`--append-system-prompt` can
now be used in interactive mode, not just --print/-p."

## Prompt value

- Pinned text: the spec's "Prompt text" section, verbatim (Go constant, FR-007).
- Optional summary appended after a blank line, only when non-empty:

```
Current sandbox config:
- allowed domains: registry.npmjs.org, proxy.golang.org ...and 4 more
- allowed CIDRs: 192.168.1.0/24
- shadowed (secret) files: .env.production, deploy/certs
```

- Per list: max 8 items, then `...and N more`. Whole summary ≤10 lines.
- `network.allow_all: true` → `- network: unrestricted (allow_all)` replaces the
  domains/CIDRs lines.
- Empty lists are omitted; all empty → no summary section at all.

## Guarantees

- No file writes: workspace untouched (no `CLAUDE.md`), nothing written to
  `~/.claude/` (FR-004, SC-004).
- Startup never fails because of this feature (FR-006): every gate outcome
  starts the sandbox.
- Runtime config changes (allowlist, secrets) update the summary on next
  `kekkai up`; they never trigger an image rebuild (§6.1 — the env var is a
  runtime input; only the CMD template line is bake-time).
