# Data Model: Node Version Config

## ImageConfig (modified)

| Field | YAML key | Type | Default | Rules |
|-------|----------|------|---------|-------|
| NodeVersion | `image.node_version` | string | `lts` (constant `DefaultNodeVersion`) | Must match `^[a-z0-9.]+$`; explicit empty/blank value is a validation error (key absent → default applies) |
| AptPackages | `image.apt_packages` | []string | none | unchanged |
| ~~BaseImage~~ | ~~`image.base_image`~~ | — | — | REMOVED; entry in `legacyKeys` → migration error naming `image.node_version` |

## Derived values

| Name | Derivation | Notes |
|------|-----------|-------|
| Resolved base image | `"node:" + NodeVersion + "-" + debianRelease` | `debianRelease = "trixie"` code constant (constitution III: defaults as code constants). Never user-visible in config; appears in diagnostics and Dockerfile |
| Image tag `kekkai:<hash>` | unchanged — sha256(rendered Dockerfile + firewall script)[:12] | Resolved base image feeds the Dockerfile template where `BaseImage` did |
| `kekkai.config_hash` label | unchanged — sha256(resolved base image + apt packages + firewall script)[:12] | Same resolved string keeps hash semantics identical |

## State: pre-check outcome (transient, per `up` run)

| State | Condition | Behavior |
|-------|-----------|----------|
| Skipped (final image cached) | `docker.ImageExists(kekkai:<hash>)` | no registry contact |
| Skipped (base image local) | `docker.ImageExists(resolved base image)` | build proceeds offline |
| Confirmed exists | registry HEAD → 200 | build proceeds |
| Confirmed missing | registry HEAD → 404 | abort before build; error names `image.node_version` value (research R7) |
| Inconclusive | timeout / non-200/404 / transport error | check skipped; docker pull error is the fallback surface |

## legacyKeys (modified)

```
"image.base_image" → "image.node_version"   (NEW)
"image.base"       → "image.node_version"   (UPDATED — old replacement pointed at base_image)
```

Note: `image.base` previously mapped to `image.base_image`; its replacement text must
now point at `image.node_version` so the migration chain never recommends a dead key.

## Config key presence tracking (extended)

`imageKeysSet []string` on `Config` — populated via existing
`presentSectionKeys(data, "image")`; consumed by Validate to distinguish
`node_version: ""` (error) from absent key (default `lts`).
