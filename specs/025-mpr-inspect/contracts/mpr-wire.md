# Contract: in-sandbox capture (`kekkai-mpr` script, image, CMD)

## Image (§6.3 additions)

- `embed/kekkai-mpr.py` copied to `/usr/local/bin/kekkai-mpr`, mode 0755,
  `#!/usr/bin/env python3`, stdlib only. It is a bake-time input: enters the
  image tag hash and `kekkai.config_hash` alongside `init-firewall.sh`.
- `ENV KEKKAI_MPR_URL=http://127.0.0.1:4141` rendered from `config.MprBaseURL`.
- No new apt packages; no sudoers change; no new ports published.

## Runtime env (§5.3 addition)

`up` sets builtin `ANTHROPIC_BASE_URL=<config.MprBaseURL>` before user env.
A user `env.ANTHROPIC_BASE_URL` overrides it (docker last-value-wins); the key
is not reserved.

## CMD sequence (§6.3, §7.2)

```
startup line
&& sudo /usr/local/bin/init-firewall.sh
&& if [ "$ANTHROPIC_BASE_URL" = "$KEKKAI_MPR_URL" ]; then
       ( while :; do kekkai-mpr serve; echo "[kekkai] mpr proxy exited, restarting" >&2; sleep 1; done ) &
       kekkai-mpr wait || exit 1
   fi
&& exec claude … (unchanged)
```

`serve` prints `[kekkai] model-provider capture active (kekkai mpr)` once on
successful bind. `wait` polls a TCP connect to 127.0.0.1:4141 for up to 5 s;
on timeout prints `[kekkai] ERROR: mpr proxy failed to start` and exits 1, so
`up` fails instead of running uncaptured (FR-014). When the user overrode
`ANTHROPIC_BASE_URL`, nothing starts and nothing listens on 4141 (FR-018).

## `kekkai-mpr serve`

- Listens `127.0.0.1:4141`, HTTP/1.1 keep-alive, threaded (one thread per client connection).
- Upstream fixed: `https://api.anthropic.com` (system CA bundle). No `CONNECT`,
  no host-based routing, no other upstream — not a general relay.
- Forwards every method and path. Rewrites `Host`; strips hop-by-hop headers
  and `accept-encoding`; all other headers verbatim both ways.
- Streams the response to the client chunk by chunk while buffering a copy.
- Publishes two events per exchange on abstract unix socket `\0kekkai-mpr`
  (see below). Zero subscribers → events dropped. Per-subscriber queue 64;
  overflow drops oldest and emits a `dropped` marker.
- Body capture cap 16 MiB per body (`truncated: true` beyond); forwarding never
  affected by capture.
- Exchange id from `/dev/shm/kekkai-mpr.seq` (read at start, written per exchange).
- Upstream failure → client gets `502` with body
  `{"type":"error","error":{"type":"kekkai_mpr_upstream_error","message":"<reason>"}}`;
  the response event carries status 502 and that body.

## `kekkai-mpr wait`

Exit 0 as soon as 127.0.0.1:4141 accepts a connection; exit 1 after 5 s.

## `kekkai-mpr follow`

- Connects to `\0kekkai-mpr`, relays each event line to stdout unchanged,
  line-buffered.
- On disconnect: retries the connection every 250 ms for 10 s; on success
  emits `{"type":"notice","ts":…,"text":"mpr proxy restarted, reattached"}`
  and continues; on timeout prints `mpr proxy unavailable` to stderr, exit 1.
- Initial connect failure (proxy not running although env says capture is on)
  is treated the same way (retry window, then exit 1).

## Event lines (UTF-8 JSON, one per line)

```
{"type":"request","id":12,"ts":"2026-09-08T14:03:22.104Z","method":"POST","path":"/v1/messages?beta=true","body":"<raw request body text>","truncated":false}
{"type":"response","id":12,"ts":"2026-09-08T14:03:25.001Z","status":200,"content_type":"text/event-stream","duration_ms":2897,"body":"<raw response body text>","truncated":false}
{"type":"dropped","count":3}
{"type":"notice","ts":"…","text":"mpr proxy restarted, reattached"}
```

`body` is the raw text (`utf-8`, `errors=replace`); the host parses JSON and
reassembles SSE. No header of either direction is ever included.

## Firewall (§9)

Only change: the two `allow_all` observe-only NEW taps gain `! -o lo` so the
loopback hop does not appear in `kekkai traffic`. Verdicts unchanged; the
lockdown path already accepts loopback before its taps.
