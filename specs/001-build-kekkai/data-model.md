# Data Model: Build Kekkai

**Feature**: 001-build-kekkai | **Date**: 2026-07-06

Kekkai has no database; its "data model" is the config document, derived docker identities, and
the rendered image inputs.

## Config (`.kekkai.yaml`) — `internal/config.Config`

| Field | Type | Required | Default | Validation |
|-------|------|----------|---------|------------|
| `image.base_image` | string | yes | — | must match `node:*` |
| `image.apt_packages` | []string | no | `[]` | appended to builtin set |
| `claude.version` | string | no | `latest` | `latest` or exact npm version |
| `claude.args` | string | no | `--dangerously-skip-permissions` | replaces default verbatim |
| `git.enabled` | bool | no | false | — |
| `git.ssh_agent` | bool | no | false | requires `git.enabled: true`; at `up` requires host `$SSH_AUTH_SOCK` |
| `disk.mounts` | []Mount | no | `[]` | see Mount |
| `env` | map[string]string | no | `{}` | reserved keys rejected: `WORKSPACE`, `ALLOW_ALL`, `ALLOW_GITHUB`, `ALLOWED_DOMAINS`, `ALLOWED_CIDRS`, `SSH_AUTH_SOCK` |
| `network.allow_all` | bool | no | false | exclusive of every other network key |
| `network.allow_github` | bool | no | false | — |
| `network.allowed_cidrs` | []string | no | `[]` | each parses as CIDR |
| `network.allowed_domains` | []string | no | `[]` | no whitespace |
| `secrets.hide` | []string | no | `[]` | exact paths relative to workspace root, no globs |
| `limits.cpus` | number | no | unset | positive |
| `limits.memory` | string | no | unset | matches `^[0-9]+(\.[0-9]+)?[bkmg]?$` case-insensitive (docker `--memory` grammar, e.g. `8g`) |

Document-level rules (§4.1, §4.4):

- Exactly one of `.kekkai.yml`/`.kekkai.yaml`; both → error; none → error pointing at `kekkai init`.
- Unknown keys rejected (`KnownFields`); legacy keys (`image.base`, `image.claude_code_version`,
  `firewall`, `docker_access`, top-level `mounts`) → targeted schema-changed error.
- All violations reported in one pass before any docker work.

### Mount

| Field | Type | Required | Semantics |
|-------|------|----------|-----------|
| `source` | string | yes | `~`/`${VAR}` expanded; unset `${VAR}` errors unless `optional` |
| `target` | string | no | inferred: `~/x` → `/home/kekkai/x`; absolute source → same path (§4.3); explicit wins; duplicates across mounts rejected |
| `readonly` | bool | no | — |
| `optional` | bool | no | missing source: skip + notice (else warn) |

## Derived identity (`internal/runtime/identity.go`)

| Value | Formula | Used by |
|-------|---------|---------|
| Container name | `kekkai-<sanitized-basename(PWD)>-<sha256(PWD)[:8]>`; sanitized = lowercase, chars outside `[a-z0-9_.-]` → `-` | up |
| Authoritative key | label `kekkai.cwd=<PWD>` | up, down, shell, ps, prune |
| Extra labels | `kekkai.image_hash`, `kekkai.version` | ps, prune |
| History volume | `kekkai-history-<sha256(PWD)[:8]>` | up, prune --volumes |
| Image tag | `kekkai:<sha256(rendered Dockerfile + init-firewall.sh)[:12]>` | up, prune |
| Config hash (image label `kekkai.config_hash`) | `sha256(base_image + "\n" + apt packages + "\n" + init-firewall.sh)[:12]` — version-independent; keys the §6.2 offline fallback only, never builds | up |

## Image bake inputs (§6.1) — nothing else enters the hash

- `image.base_image`
- builtin apt packages (§5.1) + `image.apt_packages`
- resolved `claude.version` (latest → concrete via npm registry, §6.2)
- `embed/init-firewall.sh` content

## Container run inputs (runtime only, never hashed — §7.3 assembly order)

1. Caps: `NET_ADMIN`, `NET_RAW` (fixed)
2. Builtin mounts: `$PWD→/workspace` rw, `~/.claude→/home/kekkai/.claude` rw, history volume
   → `/commandhistory`
3. Git mounts per `git.*` state (§5.2)
4. `disk.mounts`
5. Secrets shadows: file → `/dev/null` ro bind, dir → tmpfs, missing → warn+skip (§8)
6. Env: builtin (§5.3) → user `env` → firewall env (authoritative last) → `CLAUDE_ARGS`
7. `limits` → `--cpus`/`--memory`
8. Workdir `/workspace`

## Firewall env contract (§9)

| Var | Source | Value |
|-----|--------|-------|
| `ALLOW_ALL` | `network.allow_all` | `1`/unset |
| `ALLOW_GITHUB` | `network.allow_github` | `1`/unset |
| `ALLOWED_DOMAINS` | `network.allowed_domains` | space-separated |
| `ALLOWED_CIDRS` | `network.allowed_cidrs` | space-separated |

## State transitions

```
(no container) --kekkai up--> running --claude exit / SIGINT / SIGTERM / kekkai down--> removed (--rm)
running --kekkai up--> refused (unless --force: remove, recreate)
```

Image lifecycle: built on hash miss; unreferenced `kekkai:*` images removed only by `prune`.
