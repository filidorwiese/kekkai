# Contract: `image.node_version` config key

**Feature**: 014-debian-nvm-base

Validation happens at config-parse time, before any docker activity, reported in the single one-pass violation list (§4.4).

## Accepted values

| Input | Meaning | nvm argument |
|---|---|---|
| key absent | default | `--lts` (via default `lts`) |
| `lts` | latest LTS at build time | `--lts` |
| `22` | newest 22.x.y | `22` |
| `22.11` | newest 22.11.x | `22.11` |
| `22.11.0` | exactly 22.11.0 | `22.11.0` |

Pattern: `^(lts|[0-9]+(\.[0-9]+){0,2})$`

## Rejected values → exact error behavior

| Input | Error (must contain) |
|---|---|
| `""` (explicit empty) | `image.node_version must not be empty (omit the key for the default "lts")` |
| `node`, `stable`, `current`, `lts/*`, `lts/jod`, `v22`, `22-slim`, any other non-matching string | error naming the key and the accepted forms: `lts`, major (`22`), major.minor (`22.11`), full (`22.11.0`) |
| `image.base_image` present (any value) | legacy-key migration error naming `image.node_version` as the replacement (existing `legacyKeys` behavior, preserved) |
| `image.base` present | same migration error (existing, preserved) |

Guarantees:

- Zero docker calls before these errors are reported (SC-003).
- All violations reported together with any other config errors (one pass).

## Well-formed but nonexistent versions

`99`, `22.99.0` etc. pass validation (existence is unknowable at parse time) and fail at image build; the build output's final error line names `image.node_version` — see [dockerfile.md](dockerfile.md).

## Removed behavior

- `current`, node codenames, and arbitrary `[a-z0-9.]+` selectors: previously accepted (as node:* tags), now rejected.
- Docker Hub tag-existence pre-check (`baseImageMissing`): deleted; no network I/O for version checking at `up`.
