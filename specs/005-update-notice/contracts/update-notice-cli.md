# Contract: Update Notice on `kekkai up`

## Output

Exactly one line on **stdout**, printed after image ensure/preflight and immediately
before the interactive handoff, only when a newer release exists and the check
completed in time:

```
A new version of kekkai is available (<installed> -> <latest>), run 'kekkai self-update' to upgrade
```

`<installed>` and `<latest>` are the tag forms (e.g. `v1.2.0 -> v1.3.0`).

## Silence table (no update-related output of any kind)

| Condition | Behavior |
|-----------|----------|
| Installed == latest | silent |
| Installed > latest (ahead) | silent |
| Dev build (`dev` / no `v` prefix) | silent, no network call |
| Release endpoint unreachable / timeout | silent |
| HTTP 403/429 (rate limit) or any non-200 | silent |
| Malformed response / missing tag | silent |
| Check not finished when handoff begins | silent this run |
| `up` fails for any reason (config, docker, preflight) | silent — notice never appears in/around error output |

## Guarantees

- Exit status of `up` is never affected by the check.
- No blocking: the check is read non-blockingly at the print point; startup adds no wait.
- Source of truth identical to `kekkai self-update` (same endpoint, same `KEKKAI_REPO` override, same comparison) — the advertised version is always installable by that command.
