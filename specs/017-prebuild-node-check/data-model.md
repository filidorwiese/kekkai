# Data Model: Pre-Build Node Version Existence Check

**Feature**: 017-prebuild-node-check | **Date**: 2026-07-10

No persistent data; entities are the external index, a tri-state outcome, and two existing structures that change shape.

## Entities

### 1. Node release index (external, read-only)

| Aspect | Value |
|---|---|
| URL | `https://nodejs.org/dist/index.json` (same dataset as nvm's `index.tab`, research.md R1) |
| Shape | JSON array; only `version` field consumed (`"vX.Y.Z"`) |
| Access | at most one GET per build-triggering `up`; 10s timeout, no retries, no caching |
| Consumers | new probe `nodeVersionMissing` (internal/runtime/up.go) |

### 2. Existence-check outcome (tri-state, transient)

| Outcome | Condition | Effect |
|---|---|---|
| confirmed absent | 200 + parsed + no selector match | abort before build; contract error naming `image.node_version` |
| confirmed present | 200 + parsed + match | proceed to build |
| inconclusive | timeout / transport error / non-200 / parse failure | proceed to build, silent |

Selector → match rule: `x.y.z` exact `v<sel>`; `x`/`x.y` prefix `v<sel>.`; `lts` never checked.

### 3. Dockerfile template data (`internal/runtime/up.go`, changed)

| Field | Before | After |
|---|---|---|
| `NodeVersionRaw` | rendered into the in-build ERROR wrap | **removed** (wrap gone, field unused) |
| All others (`DebianImage`, `NvmVersion`, `NodeInstallArg`, `AptPackages`, `ClaudeVersion`) | unchanged | unchanged |

### 4. Node install build step (`embed/Dockerfile.tmpl`, changed)

| Aspect | Before | After |
|---|---|---|
| Failure handling | `\|\| { echo "ERROR: Node version ... check image.node_version ..."; exit 1; }` | none — nvm's native error is the only message |
| Install/alias/symlink chain | — | unchanged |
| Rendered output | — | changes → image hash changes → one-time rebuild on next `up` |

**Relationships**: `ensureImage` image-miss branch → (numeric selector) → probe → tri-state outcome → abort/build. Cached-image path never reaches the probe. No state survives the run.
