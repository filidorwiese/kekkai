# Data Model: Inspect model-provider requests (`kekkai mpr`)

Nothing is persisted on disk. Proxy state lives in the `kekkai-mpr serve`
process; reader state lives in one `kekkai mpr` host process. The only
file is the sequence counter in tmpfs.

## In-sandbox (proxy)

### Exchange (in flight)

One forwarded HTTP request/response pair.

| Field | Source | Notes |
|---|---|---|
| `id` | sequence counter | monotonic per sandbox lifetime, survives proxy restart via `/dev/shm/kekkai-mpr.seq` |
| `ts_request` | wall clock at forward time | ISO-8601 with ms, UTC |
| `method`, `path` | client request line | path includes query string |
| `request_body` | client body bytes → text | capped at 16 MiB, `truncated` flag |
| `status` | upstream status | `502` when upstream failed |
| `content_type` | upstream header | drives SSE reassembly on the host |
| `response_body` | upstream body bytes → text | streamed to client while buffered; capped, `truncated` flag |
| `duration_ms` | response complete − request forwarded | |

Headers are never stored (FR-012).

### Subscriber

One connected `follow` client on the abstract unix socket `\0kekkai-mpr`.

| Field | Notes |
|---|---|
| queue | bounded, 64 events; overflow drops oldest and records a drop count |
| dropped | pending count, flushed as a `dropped` event before the next real event |

Lifecycle: connect → receive events from now on → disconnect (reader exit or
proxy exit). No subscriber = events discarded on publish (FR-004).

## Wire events (JSON lines, proxy → follow → host)

| `type` | Fields | When |
|---|---|---|
| `request` | `id`, `ts`, `method`, `path`, `body`, `truncated` | after the request body is fully read and forwarded |
| `response` | `id`, `ts`, `status`, `content_type`, `duration_ms`, `body`, `truncated` | after the last response byte reached the client |
| `dropped` | `count` | before the next event following overflow |
| `notice` | `text` | emitted by `follow` itself: `mpr proxy restarted, reattached` |

`body` is the raw text; the host parses JSON / SSE. Exact shapes in
[contracts/mpr-wire.md](contracts/mpr-wire.md).

## Host-side (`kekkai mpr`)

### Rendered exchange

The host reassembles what it prints from wire events; no cross-event state is
required except color/TTY mode and the `--raw` flag. Request and response
blocks print independently (clarified: request at send time, response at
completion), linked by `#<id>`.

### Message / content block (rendering input)

Parsed from `body` JSON of `/v1/messages` requests and responses.

| Block type | Rendered as |
|---|---|
| `text` | the text verbatim |
| `tool_use` | `tool_use <name> <id>` + pretty-printed `input` |
| `tool_result` | `tool_result <tool_use_id> [is_error]` + content (text blocks joined, or string) |
| `thinking` | `thinking:` + text |
| `redacted_thinking` | `[redacted thinking]` |
| `image` / `document` (base64 source) | `[image <media_type> <size>]` / `[document <media_type> <size>]` |
| other | `[<type>]` + compact JSON |

### Reassembled streamed response

SSE `text/event-stream` bodies fold into one message object with the same
shape as a non-streamed `/v1/messages` response: `id`, `model`, `role`,
`content[]`, `stop_reason`, `usage` (input tokens, cache read/write, output
tokens). An `error` event yields `{"type":"error","error":{…}}`.

## Constants (code, not config)

| Name | Value | Where |
|---|---|---|
| `config.MprBaseURL` | `http://127.0.0.1:4141` | Go; rendered into `ENV KEKKAI_MPR_URL`; builtin env `ANTHROPIC_BASE_URL` |
| upstream | `https://api.anthropic.com` | script constant, the only upstream |
| socket | abstract `\0kekkai-mpr` | script |
| sequence file | `/dev/shm/kekkai-mpr.seq` | script |
| body cap | 16 MiB | script |
| subscriber queue | 64 events | script |
| readiness timeout | 5 s | script `wait` |
| follow reconnect window | 10 s | script `follow` |
