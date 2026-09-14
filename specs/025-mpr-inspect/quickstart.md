# Quickstart: validating `kekkai mpr`

End-to-end validation against a real docker daemon (constitution IV).
Contracts: [contracts/mpr-cli.md](contracts/mpr-cli.md),
[contracts/mpr-wire.md](contracts/mpr-wire.md).

## Prerequisites

- Docker daemon; `git`, `curl` on host; a working Claude login in `~/.claude`.
- Build: `task build` (→ `/tmp/kekkai`) or `go build -o /tmp/kekkai ./cmd/kekkai`.
- Test project dir; sandbox started under a pseudo-TTY
  (`script -qec "/tmp/kekkai up" /dev/null </dev/null &`, poll `docker ps`),
  or interactively in a second terminal. First `up` rebuilds the image
  (new embedded script + Dockerfile text are bake-time inputs) — expected.

## Scenario 0 — spike (run before implementing the rest)

Prove the transport assumption (research R2): with `ANTHROPIC_BASE_URL`
pointed at the in-sandbox proxy, an OAuth-authenticated claude still works.
Minimal path: image with the script + CMD wiring only, `kekkai up`, type one
prompt, get a reply. Failure here stops the feature until resolved.

## Scenarios

`mpr` runs in one terminal (`/tmp/kekkai mpr > m.log` backgrounded for
scripted runs); claude is driven interactively in the sandbox terminal, or
requests are generated from a third terminal with
`kekkai exec curl -s http://127.0.0.1:4141/v1/models -H "x-api-key: …"`
(any authenticated call through the proxy yields an exchange).

| # | Action | Expect |
|---|---|---|
| 1 | Sandbox with default config; `mpr`; send a prompt to claude | `==== #N REQUEST … POST /v1/messages` block with the prompt text in a `[user]` block, then `==== #N RESPONSE … 200` with the reply (US1 sc. 1-2, SC-001) |
| 2 | Prompt that triggers a tool (e.g. "read README.md") | `tool_use Read …` in a response block; `tool_result …` in the next request block (US1 sc. 3) |
| 3 | Same as 1, but the next request | Full system prompt, all tool definitions, whole conversation repeated (FR-008, clarified) |
| 4 | `mpr` started only after several turns | Only new exchanges appear, no replay (FR-010) |
| 5 | Two `mpr` sessions at once, one prompt | Both print the same `#N` blocks (FR-016) |
| 6 | Ctrl+C `mpr` (pty `\003` trick, feature 009) | Exit 0; `docker exec <id> pgrep -f 'kekkai-mpr follow'` empty; claude unaffected (US2 sc. 3, SC-006) |
| 7 | `mpr` in a dir without sandbox | `no running sandbox for <pwd>, run 'kekkai up'`, exit 1 |
| 8 | `mpr extra` | `usage: kekkai mpr [--raw]`, exit 1 |
| 9 | `mpr --raw` during a prompt | One JSON object per line; response `body` is a message object with `content[]`, `stop_reason`, `usage` — no SSE frames; `grep -c '\x1b\['` = 0 |
| 10 | `mpr > m.log` then `grep -c $'\x1b\[' m.log` | 0 — no color codes when piped; run on a TTY shows colored separators |
| 11 | `grep -o 'sk-ant-[A-Za-z0-9_-]*' m.log`; also grep the OAuth token prefix | No matches in rendered or raw output (FR-012, SC-003) |
| 12 | `kekkai exec pkill -f '^python3 /usr/local/bin/kekkai-mpr serve'` while `mpr` attached (the anchored pattern spares the supervisor subshell, whose cmdline is the whole CMD string), then prompt again | Claude terminal shows `[kekkai] mpr proxy exited, restarting`; `mpr` prints `---- notice … mpr proxy restarted, reattached ----`; next exchange captured with a higher `#N` (FR-019, SC-007) |
| 13 | `.kekkai.yaml` with `env: {ANTHROPIC_BASE_URL: https://api.anthropic.com}`, `down`/`up`, run `mpr` | `capture disabled: ANTHROPIC_BASE_URL is set in .kekkai.yaml`, exit 1; `docker exec <id> ss -ltn` shows nothing on 4141; claude works (FR-018) |
| 14 | Container started from a pre-feature image/binary (if available), `mpr` | `sandbox predates 'kekkai mpr' …` or image-predates hint, exit 1 |
| 15 | `kekkai traffic` alongside a prompt (firewall mode) | No `127.0.0.1:4141` lines; `ALLOW tcp <ip>:443 (api.anthropic.com)` still appears |
| 16 | Same with `network.allow_all: true` | No `127.0.0.1:4141` lines either (R7 `! -o lo`) |
| 17 | `down`; `grep -r "<a distinctive prompt string>" ~/.cache/kekkai /tmp 2>/dev/null`; `docker history <image>` | No transcript content on host or in image layers (SC-005) |
| 18 | Long session (30+ turns) without `mpr` attached; `docker stats` | Proxy RSS flat; nothing accumulates (FR-004, SC-004) |
| 19 | Time a small `count_tokens` call via `kekkai exec curl` against `http://127.0.0.1:4141` vs `https://api.anthropic.com` (10 runs each) | Median difference < 50 ms (SC-002) |
| 20 | Regression: fresh `up` output | Startup line, firewall probes, `[kekkai] model-provider capture active`, then claude; `kekkai shell`/`exec`/`traffic` unchanged |

## Cleanup

```sh
docker rm -f $(docker ps -q --filter label=kekkai.cwd=$PWD)
docker volume rm <kekkai-history-hash-of-THIS-test-dir-only>   # never bulk-remove kekkai-history-*
```
