# Research: Inspect model-provider requests (`kekkai mpr`)

Refines the spec's decisions (always-on in-sandbox interception via
`ANTHROPIC_BASE_URL`, live-only in-memory stream, full-request rendering,
bypass on user override, auto-restart) against the actual codebase:
`embed/Dockerfile.tmpl`, `embed/init-firewall.sh`, `internal/runtime/up.go`,
`internal/runtime/traffic.go`.

## R1 — Interception runtime: embedded Python script, in-sandbox

**Decision**: One embedded script `embed/kekkai-mpr.py` (stdlib only:
`http.server.ThreadingHTTPServer`, `http.client.HTTPSConnection`, `socket`,
`json`, `threading`), copied to `/usr/local/bin/kekkai-mpr` at build. It is a
plain HTTP/1.1 reverse proxy on `127.0.0.1:4141` with a single fixed upstream
`https://api.anthropic.com`. Three modes: `serve` (proxy + publisher), `wait`
(readiness probe for the CMD), `follow` (subscriber that relays JSON lines to
stdout for the host).

**Rationale**: `python3` is already a builtin package (specs/024), so no new
apt package and no new binary. The host `kekkai` binary cannot run inside the
container (darwin/arm64 hosts), and cross-compiling/embedding a second Go
binary would balloon the artifact. Python's stdlib covers threaded HTTP
serving, TLS clients with the system CA bundle (`ca-certificates` baked), and
unix sockets. The firewall script sets the precedent: sandbox-side logic is an
embedded script, host-side logic is Go.

**Alternatives considered**:
- Node script — also present, but `image.node_version` is user-configurable so
  the script would have to track a moving runtime; Python is pinned by the
  Debian base. Rejected.
- Second Go binary embedded for linux/amd64+arm64 — 2×~5 MB inside the host
  binary, cross-build matrix in release. Rejected (Principle III).
- `mitmproxy`/`socat`/`nginx` from apt — daemons, config files, TLS
  interception machinery far beyond a loopback reverse proxy. Rejected.

## R2 — Transport: `ANTHROPIC_BASE_URL` builtin env, docker last-wins override

**Decision**: `up` adds `-e ANTHROPIC_BASE_URL=http://127.0.0.1:4141` in the
builtin env block (§5.3, before user env). The constant lives in Go
(`config.MprBaseURL`) and is rendered into the image as
`ENV KEKKAI_MPR_URL=…` so the CMD and the script read one source. A user `env`
entry for `ANTHROPIC_BASE_URL` overrides for free via docker's last-value-wins
ordering (§7.3) — the spec's bypass rule (FR-018) needs no extra config key and
no validation change (`ANTHROPIC_BASE_URL` is deliberately not added to
`reservedEnvKeys`). In-container, the CMD starts the proxy only when
`$ANTHROPIC_BASE_URL = $KEKKAI_MPR_URL`; otherwise claude goes straight to the
user's URL and nothing listens on 4141.

**Rationale**: Claude Code's SDK honors `ANTHROPIC_BASE_URL` for every API call
(messages, count_tokens, OAuth refresh against the same host); plain `http://`
on loopback is the documented LLM-gateway pattern. Plaintext never leaves the
network namespace: the connection is process → lo → process inside the same
container. Auth headers pass through untouched; the proxy neither reads nor
stores them.

**Risk (validate first, before building the rest)**: OAuth-authenticated
(subscription) sessions plus a custom base URL. Expected to work — the base
URL changes only the host, the bearer header is unchanged — but this is the
load-bearing assumption of the feature, so quickstart scenario 0 is a manual
spike: start a sandbox with the proxy, run one prompt, confirm a reply. If it
fails, fall back to investigating `ANTHROPIC_AUTH_TOKEN` passthrough before
any further work.

## R3 — Capture stream: abstract unix socket pub/sub, no filesystem

**Decision**: The `serve` process listens on the Linux abstract unix socket
`\0kekkai-mpr`. Each `follow` client is a subscriber; every capture event
(JSON line) is written to all current subscribers. Per-subscriber bounded queue
(64 events); when full the oldest events are dropped and a
`{"type":"dropped","count":n}` marker is queued so the reader can say so. With
zero subscribers events are discarded immediately. Nothing touches a
filesystem except the sequence counter (R4).

**Rationale**: matches the spec's pipe semantics exactly (live-only, discard
when unread, bounded, multiple concurrent readers each seeing everything from
attach onward, gone with the container). An abstract socket needs no file, no
cleanup after a crash, no tmpfs sizing.

**Alternatives considered**:
- FIFO in `/dev/shm` — single-reader semantics, writer blocks or fails without
  a reader, multiple readers interleave fragments. Rejected.
- Ring file in `/dev/shm` + `tail -F` — rotation logic, and it is history
  storage the spec explicitly excluded. Rejected.
- Loopback TCP for subscribers — reachable by any sandbox process the same way
  the unix socket is; no gain, one more port. Rejected.

## R4 — Supervision, readiness, exchange numbering

**Decision**: CMD (still `/bin/sh -c`) after the firewall, when capture is on:
`( while :; do kekkai-mpr serve; echo '[kekkai] mpr proxy exited, restarting' >&2; sleep 1; done ) &`
then `kekkai-mpr wait || exit 1` (polls a TCP connect to 4141 for up to 5 s,
prints `[kekkai] ERROR: mpr proxy failed to start` on timeout — FR-014, fail
loud, no fallback), then `exec claude …` as today. When claude exits, PID 1
exits and the container takes the loop with it. Exchange numbers are read from
and written to `/dev/shm/kekkai-mpr.seq` (tmpfs = memory, a few bytes) so a
restarted proxy continues numbering (FR-019) — the only file the feature
touches, and it dies with the container.

**Rationale**: a shell loop is the smallest supervisor that satisfies the
clarified auto-restart choice; readiness via a probe mode keeps the CMD free of
sleep-and-hope. `follow` mode reconnects to the socket for up to 10 s when the
connection drops and emits a `notice` event on success, so the host reader
survives a restart without host-side logic.

**Alternatives considered**: `wait` by `sleep 0.5` (racy); supervisor inside
Python (fork/exec of itself — more code than three shell lines); storing the
counter in the socket dir (no dir — abstract socket).

## R5 — SSE reassembly and rendering happen on the host (Go)

**Decision**: The in-sandbox script is a dumb pipe: it captures the request
body as text and the response body as text plus `content_type`, `status`,
`duration_ms`. The Go side (`internal/runtime/mpr.go`) reassembles
`text/event-stream` bodies into the final message object (message_start →
content_block_start/delta/stop → message_delta; `input_json_delta` fragments
concatenated then parsed at block stop; `error` events surface as an error
object) and renders both the human transcript and `--raw` JSON lines.

**Rationale**: mirrors `traffic` (tcpdump dumb, Go parses). Keeps the baked
script minimal and rarely changing (every script change is an image rebuild),
puts all presentation in the host binary where TTY/color/flags live.

## R6 — Proxy HTTP details

- `Host` rewritten to `api.anthropic.com`; hop-by-hop headers
  (`connection`, `keep-alive`, `transfer-encoding`, `proxy-*`, `te`,
  `upgrade`) stripped both ways; everything else forwarded verbatim, including
  `authorization`, `x-api-key`, `anthropic-beta`, `anthropic-version`.
- `accept-encoding` is removed from the forwarded request so upstream answers
  identity-encoded: the capture can read bodies without decompressing, and
  claude's client never asked to decode something the proxy re-encoded.
- Request body read by `content-length` (chunked request bodies are also
  accepted and de-chunked; the SDK sends content-length for JSON).
- Response streamed to the client as it arrives (chunked transfer to the
  client, `flush` per chunk) while being appended to an in-memory buffer for
  the capture event; the event is published after the last byte. Streaming
  latency is unchanged for claude.
- Per-thread upstream `HTTPSConnection` reuse (ThreadingHTTPServer gives one
  thread per keep-alive client connection, which maps 1:1 to claude's own
  keep-alive), reconnect once on a stale connection. Keeps the added cost to
  parsing, not TLS handshakes (SC-002 < 50 ms).
- Capture cap: bodies above 16 MiB are stored truncated with
  `"truncated": true`; forwarding is never affected (FR-004).
- Capture never records headers. Method, path, status, timing, bodies only
  (FR-012).
- Upstream unreachable / TLS failure → `502` to claude with a JSON error body
  `{"type":"error","error":{"type":"kekkai_mpr_upstream_error","message":…}}`
  and a response event with that status, so failures are visible in the stream.

## R7 — Firewall interplay

`init-firewall.sh` already accepts loopback (`-o lo -j ACCEPT`, section 2)
before every connection NFLOG tap, so in firewall mode the loopback hop is
neither blocked nor logged by `kekkai traffic`; the proxy's own upstream
connections hit the ipset ACCEPT for `api.anthropic.com` like claude's did.
In `allow_all` mode the two observe-only NEW taps (section 0) have no loopback
exclusion, so every claude request would add an `ALLOW tcp 127.0.0.1:4141`
line: add `! -o lo` to those two NFLOG rules. Verdict-neutral (NFLOG is
non-terminating, policies stay ACCEPT), comment-plus-match change in the
sanctioned script, §9 prose updated. No other firewall change; §5.4 unchanged.

## R8 — Host-side state detection

`kekkai mpr` resolves the running container by `kekkai.cwd` label (as
`traffic`), then reads the container env via
`docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}'` (new
`docker.ContainerEnv`):
- no `ANTHROPIC_BASE_URL` at all → container was started by a pre-feature
  kekkai → `sandbox predates 'kekkai mpr'; run 'kekkai down' and 'kekkai up' to rebuild`, exit 1;
- value ≠ `config.MprBaseURL` → `capture disabled: ANTHROPIC_BASE_URL is set in .kekkai.yaml`, exit 1 (FR-018);
- else attach. `docker exec` exit 126/127 (script missing: image predates)
  → same rebuild hint as `traffic`.

## R9 — Rendering, separators, color

- Separator lines are the greppable marker `==== #<n> REQUEST …` /
  `==== #<n> RESPONSE …`, padded with `=` to 80 columns; notices use `---- notice …`.
- Full request every time (clarified): `model`, scalar params, `system`
  (string or blocks), every tool with description and compact `input_schema`,
  every message with every content block. Response: content blocks,
  `stop_reason`, `usage`.
- Binary blocks (`image`, `document` with base64 source) → one line
  `[image image/png 12.3 KB]`; the size is derived from the base64 length.
- Non-`/v1/messages` paths: separators only (endpoint + status), no body.
- Color: only when stdout is a terminal and `NO_COLOR` is unset — the existing
  `Yellow` helper's check, generalized to a small `paint(f, code, s)` helper
  that `Yellow` becomes a call to. Palette: separators cyan bold, `[user]`
  green, `[assistant]` blue, `system:` magenta, `tool_use`/`tool_result`
  yellow, errors and status ≥ 400 red, notices yellow. `--raw` is never colored.

## R10 — Hash inputs and rebuild

`embed/kekkai-mpr.py` is a bake-time input: it enters both `ImageTag` (tag
hash) and `ConfigHash` (offline-fallback key) alongside `init-firewall.sh`
(§6.1 amended). The runtime env value never enters any hash (constraint: runtime
config never rebuilds). One-time rebuild on next `up` — the same as every
image-content feature (010, 021, 024).

## R11 — Naming inside the image

Script installed as `/usr/local/bin/kekkai-mpr` (0755, `#!/usr/bin/env python3`)
so the CMD, `docker exec`, and `pkill -f 'kekkai-mpr follow'` all use one
name; `kekkai-` prefix follows the `kekkai-<name>` file convention used for
apt repo files (specs/020).
