# Tasks: Inspect model-provider requests (`kekkai mpr`)

**Input**: Design documents from `/specs/025-mpr-inspect/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/mpr-cli.md, contracts/mpr-wire.md, quickstart.md

**Tests**: No test-framework tasks — validation is end-to-end against a real docker daemon per constitution IV; each story phase carries its quickstart scenarios.

**Organization**: Tasks grouped by user story. US1 (see exchanges) and US2 (attach anytime, transparent) are both P1 and share the proxy: Phase 2 builds the transport and proves it with the spike, US1 adds capture + reader + rendering, US2 adds the attach/detach/bypass/restart edge paths, US3 (nothing persists) verifies the no-storage and no-credential properties and caps. SPECIFICATION.md is amended alongside the code (Principle I).

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Working e2e harness and the shared constants every later task uses

- [X] T001 Build test binary (`task build` → `/tmp/kekkai`) and prepare scratch project `/tmp/kekkai-qs-025` with a minimal `.kekkai.yaml` (pinned `claude.version`); confirm docker daemon reachable and `~/.claude` holds a working login
- [X] T002 [P] Add `MprBaseURL = "http://127.0.0.1:4141"` to the defaults const block in internal/config/config.go with a comment naming it a code constant (§5.3 builtin env, never config)
- [X] T003 [P] Add `//go:embed kekkai-mpr.py` → `var MprScript string` to embed/embed.go; create embed/kekkai-mpr.py with the `#!/usr/bin/env python3` shebang, a module docstring stating the three modes and constants (`UPSTREAM`, `LISTEN`, `SOCKET_NAME = "\0kekkai-mpr"`, `SEQ_FILE`, `BODY_CAP`, `QUEUE_LEN`), and a `main()` dispatching `serve|wait|follow` with stubs

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Transport in place and the load-bearing assumption proven (research.md R2): claude, with its normal login, works through the in-sandbox proxy

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Implement `serve` forward path in embed/kekkai-mpr.py: `ThreadingHTTPServer` on 127.0.0.1:4141, one handler for all methods, per-thread `HTTPSConnection("api.anthropic.com")` reused with one reconnect on stale socket, `Host` rewrite, strip hop-by-hop headers and `accept-encoding`, read request body by content-length or chunked, forward, stream response chunks to the client with per-chunk flush, HTTP/1.1 keep-alive; print `[kekkai] model-provider capture active (kekkai mpr)` once after bind; upstream failure → 502 with the `kekkai_mpr_upstream_error` JSON body (contracts/mpr-wire.md)
- [X] T005 [P] Implement `wait` mode in embed/kekkai-mpr.py: poll TCP connect to 127.0.0.1:4141 every 100 ms for 5 s; exit 0 on success, print `[kekkai] ERROR: mpr proxy failed to start` and exit 1 on timeout
- [X] T006 Wire the image in embed/Dockerfile.tmpl: `COPY kekkai-mpr.py /usr/local/bin/kekkai-mpr` + `chmod 0755` in the root block next to the firewall COPY; `ENV KEKKAI_MPR_URL={{.MprBaseURL}}`; extend CMD after the firewall with the guarded restart loop and `kekkai-mpr wait || exit 1` exactly as in contracts/mpr-wire.md, then the unchanged `exec claude` branch
- [X] T007 Host side in internal/runtime/up.go: write `assets.MprScript` into the build dir in `buildImage` (0755); add `MprBaseURL` to the template data struct in `renderDockerfile`; add `addEnv("ANTHROPIC_BASE_URL", config.MprBaseURL)` in the builtin env block before user env with a comment on the last-value-wins override (FR-018)
- [X] T008 Hash inputs in internal/runtime/identity.go: `ImageTag(rendered, firewallScript, mprScript)` and `ConfigHash(..., firewallScript, mprScript, uid, gid)` include the script text; update every caller in internal/runtime/up.go (`ensureImage`) and any other reference (`grep -rn "ImageTag(\|ConfigHash(" internal cmd`)
- [X] T009 `go build ./... && go vet ./... && python3 -m py_compile embed/kekkai-mpr.py`; then quickstart scenario 0 (spike): `kekkai up` in `/tmp/kekkai-qs-025` (expect rebuild), confirm startup prints the capture-active line, type one prompt to claude, get a reply; also run one tool-using prompt to exercise a streamed response. If the reply fails, stop and record findings in specs/025-mpr-inspect/research.md R2 before any further task

**Checkpoint**: Claude works through the proxy — story implementation can begin

---

## Phase 3: User Story 1 - See what leaves the sandbox toward the model provider (Priority: P1) 🎯 MVP

**Goal**: `kekkai mpr` streams every exchange live: full request, reassembled response, tool calls/results, errors, with separators and color

**Independent Test**: Sandbox running, `kekkai mpr` in a second terminal, one prompt → request block with the prompt text and response block with the reply within seconds (quickstart 1-3, 9, 10)

### Implementation

- [X] T010 [US1] Capture + pub/sub in embed/kekkai-mpr.py `serve`: read/write exchange id via `/dev/shm/kekkai-mpr.seq` (create at 0, lock-free single process); build `request` event after the body is read (`id, ts ISO-8601 UTC ms, method, path incl. query, body text utf-8/replace, truncated`) and `response` event after the last byte reaches the client (`id, ts, status, content_type, duration_ms, body, truncated`); enforce 16 MiB body cap for capture only; abstract unix socket listener thread on `\0kekkai-mpr`, subscriber registry with per-subscriber 64-event queue, drop-oldest + pending `dropped` count flushed before the next event, discard immediately when no subscribers; never include headers
- [X] T011 [US1] Implement `follow` mode in embed/kekkai-mpr.py: connect to `\0kekkai-mpr`, relay lines to stdout line-buffered; on disconnect or initial failure retry every 250 ms for 10 s, emit `{"type":"notice","ts":…,"text":"mpr proxy restarted, reattached"}` on success, print `mpr proxy unavailable` to stderr and exit 1 on timeout
- [X] T012 [P] [US1] Add `ContainerEnv(id string) ([]string, error)` to internal/docker/cli.go using `docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}'`
- [X] T013 [P] [US1] Generalize color in internal/runtime/up.go: extract `paint(f *os.File, code, msg string) string` (TTY + `NO_COLOR` check) and make `Yellow` call it; add the palette constants (cyan bold, green, blue, magenta, yellow, red) used by rendering
- [X] T014 [US1] Create internal/runtime/mpr.go `Mpr(raw bool) (int, error)` modeled on traffic.go: resolve running container by `LabelCwd`; `ContainerEnv` checks → `no running sandbox…`, `sandbox predates 'kekkai mpr'…`, `capture disabled: ANTHROPIC_BASE_URL is set in .kekkai.yaml` (contracts/mpr-cli.md, exit 1 each); banner `watching model-provider requests of sandbox for <pwd> (Ctrl+C to stop)` on stderr; spawn `docker exec <id> kekkai-mpr follow`, scan stdout lines into an events channel; SIGINT/SIGTERM → kill reader + `docker exec <id> pkill -f 'kekkai-mpr follow'`, exit 0; reader exit 126/127 → image-predates hint; reader stderr `mpr proxy unavailable` → relay, exit 1; other exit → `sandbox stopped`, exit 1
- [X] T015 [US1] Create internal/runtime/mpr_render.go: wire-event structs; SSE reassembly for `text/event-stream` bodies (`message_start` → content blocks via `content_block_start/delta/stop` with `text_delta`, `input_json_delta` concatenated and parsed at stop, `thinking_delta`; `message_delta` for `stop_reason`/`usage`; `error` event → error object) producing the same shape as a non-streamed message; `--raw` emitter: one JSON object per line with `type,id,ts,method,path,body` / `type,id,ts,status,duration_ms,body` / notice, `body` parsed when it parses else string, never colored
- [X] T016 [US1] Transcript rendering in internal/runtime/mpr_render.go: 80-col `==== #<n> REQUEST  HH:MM:SS <method> <path> ===…` and `==== #<n> RESPONSE HH:MM:SS <status> <dur> ===…` separators; request: `model`, scalar params line, `system:` (string or blocks), `tools (N):` each with description and compact `input_schema`, `messages:` with `[user]`/`[assistant]` and every content block per data-model.md table (`text`, `tool_use` + pretty input, `tool_result` + content, `thinking`, `redacted_thinking`, image/document → `[image <media_type> <size>]` from base64 length, other → `[<type>]` + compact JSON); response: `[assistant] <model> stop_reason=<x>`, blocks, `usage: input N (cache read N, cache write N) output N`; status ≥ 400 → `error <type>: <message>`; non-`/v1/messages` → separators only; `[truncated at 16 MiB]` trailer; `dropped` → `---- notice HH:MM:SS N events dropped (slow reader) ----`, `notice` → `---- notice HH:MM:SS <text> ----`; palette per contracts/mpr-cli.md via `paint`
- [X] T017 [US1] Dispatch in cmd/kekkai/main.go: `case "mpr"` with a `flag.NewFlagSet("mpr")` defining `--raw`; any positional arg or parse error → `usage: kekkai mpr [--raw]` exit 1; call `runtime.Mpr(*raw)`; add the three-line help entry from contracts/mpr-cli.md under `traffic`
- [X] T018 [US1] Rebuild (`task build`, `kekkai down && kekkai up` in `/tmp/kekkai-qs-025`) and run quickstart scenarios 1, 2, 3, 8, 9, 10: prompt → request/response blocks; tool round-trip visible; full system/tools/conversation repeated every request; usage error; `--raw` shape with reassembled message and no SSE frames; zero escape codes when piped, colored on a TTY

**Checkpoint**: US1 is a usable MVP: attach, watch every exchange, rendered or raw

---

## Phase 4: User Story 2 - Inspect without preparation, and without changing Claude's behavior (Priority: P1)

**Goal**: Attach/detach/re-attach at any moment with no effect on claude; bypass when the user sets `ANTHROPIC_BASE_URL`; proxy auto-restart; clean failure paths

**Independent Test**: Sandbox with empty config, several turns, then `kekkai mpr` → new exchanges only; Ctrl+C leaves claude running; second attach works; user override disables capture cleanly (quickstart 4-7, 12, 13, 14, 20)

### Implementation

- [X] T019 [US2] Verify the override path end to end: `.kekkai.yaml` `env: {ANTHROPIC_BASE_URL: https://api.anthropic.com}` → CMD skips the loop (`ss -ltn` shows nothing on 4141), claude works, `kekkai mpr` prints `capture disabled: ANTHROPIC_BASE_URL is set in .kekkai.yaml` exit 1 (quickstart 13); fix internal/runtime/mpr.go or embed/Dockerfile.tmpl if any step deviates
- [X] T020 [US2] Verify restart path (quickstart 12): kill `kekkai-mpr serve` inside the sandbox while `mpr` is attached → claude terminal shows `[kekkai] mpr proxy exited, restarting`, `mpr` prints the reattached notice, next exchange has a higher `#N` (seq file survives); fix embed/kekkai-mpr.py `follow` reconnect or seq handling if it deviates
- [X] T021 [US2] Verify attach semantics (quickstart 4, 5, 6, 7, 14): late attach shows only new exchanges; two concurrent `mpr` sessions both print the same `#N`; Ctrl+C exits 0 with no `kekkai-mpr follow` left (`pgrep -f`), claude unaffected; no-sandbox error; predates errors against an old container/image if one is available (otherwise simulate by `docker exec` env check with a manually started `debian` container labeled `kekkai.cwd`)
- [X] T022 [US2] Regression (quickstart 20): fresh `up` output order is startup line → firewall probes pass → capture-active line → claude; `kekkai shell`, `exec`, `traffic` unchanged; `--force` recreate works

**Checkpoint**: Every attach/detach/bypass/restart path behaves per contract

---

## Phase 5: User Story 3 - Nothing sensitive persists (Priority: P2)

**Goal**: Capture is memory-only and bounded; credentials never appear; `kekkai traffic` stays clean of the loopback hop

**Independent Test**: Session with `mpr`, then `down` → no transcript on host or in image layers; grep of all `mpr` output for the credential finds nothing; proxy RSS flat without readers (quickstart 11, 15-18)

### Implementation

- [X] T023 [US3] Firewall in embed/init-firewall.sh: add `! -o lo` to the two `allow_all` observe-only NEW NFLOG taps (section 0) with a comment naming the mpr loopback hop; no other rule changes; confirm the lockdown path needs nothing (lo ACCEPT precedes its taps)
- [X] T024 [US3] Verify quickstart 15 and 16: `kekkai traffic` in firewall mode and in `allow_all` mode shows no `127.0.0.1:4141` lines while claude works; `ALLOW tcp <ip>:443 (api.anthropic.com)` still appears; §9.6 probes still pass on startup
- [X] T025 [US3] Verify quickstart 11, 17, 18: grep rendered and raw output for `sk-ant-` and the OAuth token prefix → none; after `down`, grep host cache/tmp and `docker history` for a distinctive prompt string → none; 30+ turns without a reader → proxy RSS flat in `docker stats`, `/dev/shm/kekkai-mpr.seq` is the only artifact; fix embed/kekkai-mpr.py if anything is retained
- [X] T026 [US3] Verify quickstart 19 (SC-002): 10 timed `count_tokens` calls via `kekkai exec curl` against `http://127.0.0.1:4141` vs `https://api.anthropic.com`; median overhead < 50 ms; if not, fix connection reuse in embed/kekkai-mpr.py

**Checkpoint**: Security properties verified

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Spec-first amendments, docs, final sweep

- [X] T027 [P] Amend SPECIFICATION.md: §3 CLI block and bullet for `mpr` (behavior summary, `--raw`, exit codes, pointer to `specs/025-mpr-inspect/contracts/mpr-cli.md`); §5.3 builtin `ANTHROPIC_BASE_URL` with the override-disables-capture rule and the note that the key is not reserved; §6.1 script as hash input; §6.3 script install, `ENV KEKKAI_MPR_URL`, CMD sequence, pointer to `contracts/mpr-wire.md`; §7.2 CMD text; §9 `allow_all` taps exclude lo
- [X] T028 [P] README.md: `kekkai mpr` line in the Usage block; a short paragraph under the traffic/inspection docs explaining what `mpr` shows, `--raw`, and that setting `ANTHROPIC_BASE_URL` in `env` disables capture; a Known-limitations bullet that provider routes not honoring `ANTHROPIC_BASE_URL` are not captured and that captured content is visible to sandbox processes
- [X] T029 [P] Update `kekkai init` starter comments in internal/runtime/init.go: one commented `env` example line noting `ANTHROPIC_BASE_URL` points claude elsewhere and turns off `kekkai mpr` capture
- [X] T030 `gofmt -l . ; go vet ./... ; python3 -m py_compile embed/kekkai-mpr.py`; re-read embed/kekkai-mpr.py and internal/runtime/mpr_render.go for dead code and comment quality (why, not what); confirm contracts/mpr-cli.md strings match the binary byte-for-byte (`grep` each message)
- [X] T031 Final e2e sweep: full quickstart table on a fresh `down`/`up`; cleanup per quickstart (scratch container and its history volume only)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies; T002/T003 parallel
- **Foundational (Phase 2)**: depends on Phase 1; T009 spike gates everything after
- **US1 (Phase 3)**: depends on Phase 2; MVP
- **US2 (Phase 4)**: depends on US1 (needs a working reader to verify against); mostly verification with targeted fixes
- **US3 (Phase 5)**: depends on Phase 2 for T023/T024 (firewall + traffic), on US1 for T025/T026; T023 can start in parallel with Phase 3
- **Polish (Phase 6)**: depends on all stories; T027/T028/T029 parallel

### Within-story order

- US1: T010 → T011 (script), T012/T013 parallel with script work, T014 after T012, T015 after T011, T016 after T013+T015, T017 after T014, T018 last
- US2: T019-T022 any order, all after T018
- US3: T023 → T024; T025/T026 after T018

### Parallel Opportunities

- T002 ‖ T003
- T004 ‖ T005
- T010+T011 (script) ‖ T012 ‖ T013 (Go)
- T023 ‖ Phase 3 work
- T027 ‖ T028 ‖ T029

---

## Implementation Strategy

### MVP First (Phase 1-3)

1. Setup + Foundational through the T009 spike: proxy in the image, claude verified working through it. This is the risk gate; nothing else matters if it fails.
2. US1: capture, reader, rendering, dispatch. Stop and validate with quickstart 1-3, 8-10.

### Incremental Delivery

3. US2: prove attach/bypass/restart paths; small fixes as found.
4. US3: firewall tap fix, credential/persistence/overhead verification.
5. Polish: SPECIFICATION + README in the same commit as the code (Principle I), final sweep.

### Notes

- Every image-content change (script, Dockerfile, firewall) triggers a rebuild on the next `up`; batch script edits before rebuilding where possible.
- All exact strings live in contracts/mpr-cli.md and contracts/mpr-wire.md; the binary and the script must match them, not the other way round.
