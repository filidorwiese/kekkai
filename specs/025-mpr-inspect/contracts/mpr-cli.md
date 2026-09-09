# Contract: `kekkai mpr` CLI

## Invocation

```
kekkai mpr [--raw]
```

Any other argument → stderr `usage: kekkai mpr [--raw]`, exit 1.

## Preconditions (checked in this order)

| Condition | stderr | Exit |
|---|---|---|
| No running sandbox for `$PWD` | `no running sandbox for <pwd>, run 'kekkai up'` | 1 |
| Container env has no `ANTHROPIC_BASE_URL` (started by a pre-feature kekkai) | `sandbox predates 'kekkai mpr'; run 'kekkai down' and 'kekkai up' to rebuild` | 1 |
| Container env `ANTHROPIC_BASE_URL` ≠ builtin loopback URL (user override) | `capture disabled: ANTHROPIC_BASE_URL is set in .kekkai.yaml` | 1 |
| `docker exec` of the reader exits 126/127 (script missing in image) | `sandbox image predates 'kekkai mpr'; run 'kekkai down' and 'kekkai up' to rebuild` | 1 |

Startup banner (stderr, one line): `watching model-provider requests of sandbox for <pwd> (Ctrl+C to stop)`.

## Stream contract (stdout)

Only exchanges that begin after attach appear. Each block starts with a
separator line padded with `=` to 80 columns:

```
==== #12 REQUEST  14:03:22 POST /v1/messages ==================================
model: claude-opus-5
max_tokens: 32000  stream: true
system:
<system prompt text, verbatim>
tools (3):
  Read: Reads a file from the local filesystem…
    input_schema: {"type":"object","properties":{…}}
  …
messages:
[user]
<text>
[assistant]
<text>
tool_use Read toolu_01…
{
  "file_path": "/workspace/README.md"
}
[user]
tool_result toolu_01…
<content>
==== #12 RESPONSE 14:03:25 200 2.8s ===========================================
[assistant] claude-opus-5 stop_reason=tool_use
<text>
tool_use Edit toolu_02…
{ … }
usage: input 1234 (cache read 1000, cache write 0) output 56
```

- Marker: every separator line starts with `==== #<n> REQUEST ` or
  `==== #<n> RESPONSE ` (`grep '^==== #'` lists all exchanges; `grep '^==== #12 '`
  one exchange). Timestamps are `HH:MM:SS` local time.
- Response separator carries the HTTP status and wall duration; status ≥ 400
  is followed by the error body rendered as `error <type>: <message>`.
- Non-`/v1/messages` paths render the two separator lines only.
- Request rendering is complete: system, every tool definition, every
  message, every content block, every time. Nothing is deduplicated or elided.
- Binary content blocks render as `[image <media_type> <size>]` /
  `[document <media_type> <size>]`.
- Blocks of concurrent exchanges may alternate; `#<n>` links them.
- Notices (stderr-like events on stdout so they stay in sequence):
  `---- notice 14:05:01 mpr proxy restarted, reattached ----`,
  `---- notice 14:05:01 3 events dropped (slow reader) ----`.
- A truncated capture (body over the cap) ends with `[truncated at 16 MiB]`.

## Color

When stdout is a terminal and `NO_COLOR` is unset: separators cyan bold,
`[user]` green, `[assistant]` blue, `system:` magenta, `tool_use`/`tool_result`
yellow, error lines and status ≥ 400 red, notices yellow. Otherwise plain
text, no escape codes. `--raw` never emits color.

## `--raw`

One JSON object per line, in the same order as the rendered blocks:

```
{"type":"request","id":12,"ts":"2026-09-08T14:03:22.104Z","method":"POST","path":"/v1/messages","body":{…request JSON…}}
{"type":"response","id":12,"ts":"2026-09-08T14:03:25.001Z","status":200,"duration_ms":2897,"body":{…reassembled message JSON…}}
{"type":"notice","ts":"…","text":"mpr proxy restarted, reattached"}
```

`body` is the parsed JSON when the payload parses, else the raw string. A
streamed response is emitted as its reassembled final message object, never
as SSE frames.

## Exit-code contract

| Outcome | Exit | Notes |
|---|---|---|
| Ctrl+C / SIGTERM | 0 | reader killed host-side and `pkill -f 'kekkai-mpr follow'` in the sandbox |
| Sandbox stopped while attached | 1 | stderr `sandbox stopped` |
| Proxy gone for more than the reconnect window | 1 | stderr `mpr proxy unavailable` (follow gave up after 10 s) |
| Precondition failures | 1 | table above |

## Guarantees

- Observe-only: never modifies requests, responses, firewall, or container state.
- Multiple `kekkai mpr` sessions may attach concurrently; each sees every
  exchange from its own attach onward.
- No reader processes remain in the sandbox after exit.

## Help text

`kekkai help` shows:

```
  mpr         stream model-provider requests of the running sandbox for $PWD
              full request and response content, one block per exchange
              flags: --raw (JSON lines instead of rendered transcript)
```
