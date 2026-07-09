# Quickstart: validating sandbox-awareness context

End-to-end validation against a real docker daemon (constitution IV). Contract:
[contracts/sandbox-prompt.md](contracts/sandbox-prompt.md). Sandbox interaction
via `kekkai exec` (feature 009); `up` under pseudo-TTY (project memory).

## Prerequisites

- Build: `go build -o /tmp/kekkai-test ./cmd/kekkai`
- Test project dir. First `up` rebuilds the image (CMD is a bake-time input) — expected.
- Snapshot for the no-writes check: `find . -newer /tmp/stamp` baseline + `ls -laR ~/.claude | md5sum` before starting.

## Scenarios

| # | Action | Expect |
|---|---|---|
| 1 | Fresh `up`; `kekkai exec sh -c 'echo $KEKKAI_SANDBOX'` | `1` (SC-003) |
| 2 | `docker inspect <container>` env | `KEKKAI_SYSTEM_PROMPT` present, value starts `You are running inside Kekkai` and matches the spec text byte-for-byte apart from the summary section (FR-002) |
| 3 | `kekkai exec sh -c 'ps ax -o args \| grep [a]ppend-system-prompt'` (or inspect `/proc/1/...` descendants) | running claude process has `--append-system-prompt` with intact multiline text — no word-splitting (delivery integrity) |
| 4 | Interactive: ask Claude "are you running in a sandbox?" (manual, once) | answer references Kekkai (SC-001) |
| 5 | Interactive: "fetch https://some-blocked-domain.example and summarize" (manual, once) | Claude proposes `network.allowed_domains` + `kekkai up` restart, no proxy/mirror workaround (SC-002) |
| 6 | Config with `allowed_domains: [registry.npmjs.org]`, `secrets.hide: [.env.production]` (file exists); inspect env | summary section lists the domain and the shadowed name, ≤10 lines (FR-008) |
| 7 | Config with 12 allowed domains | summary shows first 8 + `...and 4 more` (truncation rule) |
| 8 | `network.allow_all: true` | summary line `network: unrestricted (allow_all)`; prompt still injected |
| 9 | `claude.version: 1.0.40` (pre-threshold pin) + `up` | one yellow warning that context is skipped; container starts; `KEKKAI_SYSTEM_PROMPT` absent from env; claude runs (US3) |
| 10 | User `claude.args: "--dangerously-skip-permissions --model claude-sonnet-5"` | args visible on claude process alongside kekkai's append flag (FR-005) |
| 11 | `up ... -- --verbose`-style extra args after `--` | still appended, sandbox starts (FR-005) |
| 12 | After scenarios: workspace `find . -newer /tmp/stamp` and `~/.claude` checksum | no `CLAUDE.md` created/modified; no kekkai-written files in `~/.claude` (SC-004; claude's own session files in `~/.claude` are expected — compare against a control run without the feature if ambiguous) |
| 13 | Regression: full `up` output | §9.6 firewall probes pass; missing-config warning and update notice unchanged |

## Cleanup

```sh
docker rm -f $(docker ps -q --filter label=kekkai.cwd=$PWD)
docker volume rm <kekkai-history-hash-of-THIS-test-dir-only>   # never bulk-remove kekkai-history-*
```
