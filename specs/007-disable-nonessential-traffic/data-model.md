# Data Model: Disable Nonessential Traffic

No persistent data; kekkai stays stateless. Two builtin sets change.

## Builtin sandbox environment (§5.3)

| Variable | Value | New? |
|----------|-------|------|
| `CLAUDE_CONFIG_DIR` | `/home/kekkai/.claude` | existing |
| `NODE_OPTIONS` | `--max-old-space-size=4096` | existing |
| `POWERLEVEL9K_DISABLE_GITSTATUS` | `true` | existing |
| `WORKSPACE` | `<basename $PWD>` | existing |
| `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` | `1` | **added** |

Ordering invariant (§7.3): builtin env → user env → firewall env. Docker
applies the last occurrence of a repeated `-e`, so a user `env:` entry for
the same variable overrides the builtin (FR-002); firewall vars stay last
and authoritative.

## Builtin always-allowed destinations (§5.4)

| Destination | Tier | Fate |
|-------------|------|------|
| `api.anthropic.com` | fatal (probe target) | kept |
| `statsig.anthropic.com` | warn | **removed** |
| `host.docker.internal` | warn (macOS host parity) | kept |

## Derived invariant

`embed/init-firewall.sh` is an input to both the image hash (§6.1) and the
`kekkai.config_hash` label; removing the statsig line changes both, so every
existing image misses on next `up` and rebuilds with the new firewall. No
migration logic needed.
