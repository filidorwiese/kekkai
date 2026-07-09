# Data Model: Debian Base Image with Build-Time Node Install (nvm)

**Feature**: 014-debian-nvm-base | **Date**: 2026-07-09

No persistent storage; the "data" is config keys, code constants, template inputs, and docker-side identity derived from them.

## Entities

### 1. `image.node_version` (config key)

| Aspect | Value |
|---|---|
| Location | `ImageConfig.NodeVersion` (`internal/config/config.go`) |
| Type | string |
| Default | `"lts"` (`DefaultNodeVersion`, applied when key absent) |
| Accepted values | `lts` \| major (`22`) \| major.minor (`22.11`) \| full (`22.11.0`) |
| Validation | `^(lts|[0-9]+(\.[0-9]+){0,2})$` at config-parse time (`validate.go`); explicit empty string → error telling user to omit the key |
| Rejected values | `node`, `stable`, `current`, `lts/*`, `lts/<codename>`, anything with `-`/whitespace — error names the four accepted forms |
| Role | Bake-time input: enters the rendered Dockerfile → changes image hash → triggers rebuild |

**State note**: `lts` is frozen at build time. The selector string (not the resolved version) enters the hash, so a newer LTS release does NOT invalidate an existing image.

### 2. Platform constants (code, kekkai-owned)

| Constant | Value | Location | Role |
|---|---|---|---|
| Debian base image | `debian:trixie` | `internal/config/config.go` (replaces `debianRelease`) | Bake-time input; changed only via kekkai release |
| nvm release tag | `v0.40.5` | same file, adjacent | Bake-time input; pins installer URL + checked-out nvm; never `master`/`latest` |

Both render into the Dockerfile, so bumping either changes every image hash (correct: platform change = rebuild).

### 3. Selector translation (derived, not stored)

| `image.node_version` | `nvm install` argument |
|---|---|
| `lts` | `--lts` |
| `22` / `22.11` / `22.11.0` | verbatim |

Exposed as a method on `ImageConfig` (e.g. `NodeInstallArg()`), replacing the deleted `ResolvedBaseImage()`.

### 4. Dockerfile template data (render-time struct, `internal/runtime/up.go`)

| Field | Source | Old equivalent |
|---|---|---|
| `DebianImage` | platform constant | `BaseImage` (was `node:<ver>-<release>`) |
| `NvmVersion` | platform constant | — (new) |
| `NodeInstallArg` | translated selector | — (new) |
| `NodeVersionRaw` | `image.node_version` verbatim | — (new; used in the build-failure message) |
| `AptPackages` | builtin (`+bash`) + user `image.apt_packages` | unchanged |
| `ClaudeVersion` | resolved `claude.version` | unchanged |

### 5. Sandbox image (docker-side)

| Aspect | Value |
|---|---|
| Tag | `kekkai:<sha256(rendered Dockerfile + init-firewall.sh)[:12]>` — mechanism unchanged (`identity.go`) |
| `kekkai.config_hash` label | inputs become: Debian base + nvm tag + node selector + apt packages + firewall script (was: resolved node:* base + apt + firewall). Same purpose: §6.2 offline fallback keying only |
| Filesystem contract | see [contracts/dockerfile.md](contracts/dockerfile.md) |
| User | `kekkai` UID/GID 1000, home `/home/kekkai`, shell zsh (created, no longer renamed from `node`) |
| Node artifacts | `/home/kekkai/.nvm/**` owned by `kekkai`; stable dir symlink `/home/kekkai/.nvm/current`; root-owned symlinks `/usr/local/bin/{node,npm,npx,claude}` |

**Relationships**: `image.node_version` + platform constants + apt packages + claude version → rendered Dockerfile → image tag (1:1). Runtime config (mounts/env/network/secrets/limits) never touches any of these.

**Migration/compat**: old `kekkai:*` images keyed by old-style hashes simply miss on inspect → next `up` rebuilds (expected per spec edge case). `image.base_image` remains in the `legacyKeys` map → parse-time migration error naming `image.node_version` (already implemented, behavior preserved).
