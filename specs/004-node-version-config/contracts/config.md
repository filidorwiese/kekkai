# Contract Delta: `.kekkai.yaml` image section

Supersedes the `image` portion of `specs/001-build-kekkai/contracts/config.md`.

## Schema

```yaml
image:
  # Node.js version for the sandbox. Resolved internally to the official
  # Debian-based Node image (pinned Debian release). Must exist as a
  # published node image tag.
  node_version: lts              # default; e.g. 24, 24.3.0, lts, current

  apt_packages: [golang]         # unchanged
```

## Validation contract (single-pass, pre-docker)

| Input | Outcome |
|-------|---------|
| key absent | default `lts` applied, valid |
| `node_version: 24` / `24.3.0` / `lts` / `current` / `jod` | valid (format) |
| `node_version: ""` (explicit) | error: `image.node_version must not be empty (omit the key for the default "lts")` |
| contains `:` `/` `-` whitespace or uppercase (e.g. `node:24`, `24-trixie`) | error: `image.node_version must be a plain version selector like "24" or "lts", got "<value>"` |
| `base_image:` present (any value) | error: `config schema changed: "image.base_image" is now image.node_version — run 'kekkai init' and see the README` |
| `base:` present (pre-rewrite key) | same legacy error style, replacement text `image.node_version` |

Exact error strings are illustrative except the legacy-key format, which follows the
established `legacyKeys` message template in `internal/config/config.go`.

## Startup contract (image acquisition, `kekkai up`)

| Precondition | Behavior |
|--------------|----------|
| final `kekkai:<hash>` image exists locally | no registry existence check, no build |
| base image (`node:<v>-trixie`) exists locally | existence check skipped, build proceeds |
| registry reachable, tag missing (404) | abort before build/pull: `image.node_version: "<v>" has no published base image (node:<v>-trixie not found on Docker Hub) — pick a version from https://hub.docker.com/_/node` |
| registry reachable, tag exists | build/pull proceeds |
| registry unreachable / inconclusive response | check skipped silently (verbose may note it); docker pull failure is the fallback error surface |

## Compatibility

- `node_version: 24` produces the identical image (same hash) as the previous
  `base_image: node:24-trixie` with an otherwise unchanged config — no rebuild on
  faithful migration.
- No other config section changes.
