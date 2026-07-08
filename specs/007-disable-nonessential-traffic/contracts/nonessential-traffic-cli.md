# Contract: Disable Nonessential Traffic

## Sandbox environment (`kekkai up`)

Every sandbox container starts with:

```
CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
```

Observable from the host:

```sh
docker exec <container> env | grep CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC
# -> CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
```

Override: a user config with

```yaml
env:
  CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: "0"
```

results in `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=0` in the container
(user env wins over builtin). No warning is printed either way.

## Firewall builtin allowlist

| Startup log line | Before | After |
|------------------|--------|-------|
| `[kekkai] allowed: api.anthropic.com (...)` | present | present |
| `[kekkai] allowed: statsig.anthropic.com (...)` | present | **absent** |
| `[kekkai] allowed: host.docker.internal (...)` | macOS only | macOS only (unchanged) |

From inside a default sandbox:

- `curl https://api.anthropic.com` succeeds (verification probe, unchanged)
- `curl https://statsig.anthropic.com` is blocked (icmp-admin-prohibited)

Verification probes (§9.6) unchanged: example.com must be blocked,
api.anthropic.com must be reachable; failure of either aborts startup.

## Image rebuild

First `up` after upgrading kekkai rebuilds the sandbox image (firewall
script is a bake-time hash input). One-time cost per config; no user action.

## Documentation

`grep -ri statsig README.md SPECIFICATION.md` and a fresh `kekkai init`
output contain zero matches. SPECIFICATION §5.4 lists `api.anthropic.com`
as the only always-allowed external destination.

## Unchanged behavior

- User `network` config (`allow_all`, `allow_github`, `allowed_domains`,
  `allowed_cidrs`) untouched; re-allowing statsig via `allowed_domains` works
  like any other domain.
- Exit codes, mounts, limits, warning lines from feature 006: unchanged.
