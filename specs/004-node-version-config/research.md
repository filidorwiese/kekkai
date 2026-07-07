# Research: Node Version Config

## R1: How to check tag existence on Docker Hub without pulling

**Decision**: Direct HTTPS against Docker Hub's registry API from the host:
1. `GET https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/node:pull` → anonymous bearer token
2. `HEAD https://registry-1.docker.io/v2/library/node/manifests/<version>-trixie` with
   `Accept: application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.index.v1+json`
- `200` → exists; `404` → confirmed missing (clean error); any other status, timeout, or transport error → inconclusive, skip check (pull is fallback)
- 10s `http.Client` timeout, mirroring `resolveLatest()` (npm) in `internal/runtime/up.go`

**Rationale**: Deterministic status codes separate "confirmed missing" (fail) from
"can't tell" (proceed) — exactly the semantics the clarification requires. Zero new
dependencies; identical shape to the existing npm registry call.

**Alternatives considered**:
- `docker manifest inspect node:X-trixie`: respects daemon config, but distinguishing
  not-found from network failure requires parsing stderr strings across runtime
  implementations (docker/podman/OrbStack) — brittle, violates the capability-based
  (never per-runtime) principle.
- Docker Hub v2 "hub" API (`hub.docker.com/v2/repositories/library/node/tags/...`):
  simpler (no token dance) but is Hub's web API, not the registry protocol; less stable
  contract.
- No pre-check: rejected by clarification (2026-07-07).

## R2: When to run the pre-check

**Decision**: Inside `ensureImage`, only on the build path — after the final
`kekkai:<hash>` tag missed `docker.ImageExists(tag)` and only when
`docker.ImageExists(resolvedBaseImage)` is also false locally.

**Rationale**: If the final image exists there is nothing to pull. If the base image is
already local, docker build won't hit the registry (and offline use must keep working —
spec edge case). Check is therefore reached exactly when a pull would happen.

**Alternatives considered**: During config `Validate()` — rejected: validation must
stay pure/offline and complete before docker work (§4.4); network I/O there breaks the
one-pass-report contract and offline `up` with cached images.

## R3: Version selector validation format

**Decision**: `node_version` must match `^[a-z0-9.]+$` (non-empty). Rejects `:`,
`/`, `-`, whitespace, uppercase.

**Rationale**: Covers everything Docker Hub publishes as `node:<X>-trixie` variants:
major (`24`), major.minor (`24.3`), full (`24.3.0`), channels (`lts`, `current`), and
LTS codenames (`jod`). Rejecting `-` prevents `24-trixie` input from doubling the
internal suffix. Existence is intentionally NOT validated here (R2).

**Alternatives considered**: Strict enum/semver pattern — rejected: would break valid
selectors (codenames, future channels) and duplicates what the existence check already
guards.

## R4: Explicit empty value vs absent key

**Decision**: Reuse the existing `presentSectionKeys(data, "image")` mechanism (already
used for `network`/`limits`) to record which image keys appear. Absent `node_version` →
default `lts` in `applyDefaults()`; present-but-empty → validation error.

**Rationale**: Spec edge case requires distinguishing the two; the pattern already
exists in `internal/config/config.go` — no new machinery.

## R5: Legacy key handling for `image.base_image`

**Decision**: Add `"image.base_image": "image.node_version"` to the `legacyKeys` map.
Existing message format: `config schema changed: "image.base_image" is now
image.node_version — run 'kekkai init' and see the README`. `KnownFields(true)` strict
decode reports it as unknown otherwise; the legacy scan upgrades it to a guided error
in the same one-pass report.

**Rationale**: Exact convention already established for `image.base`, `firewall`, etc.

## R6: Hash and rebuild semantics

**Decision**: Resolve `node_version` → `node:<v>-trixie` string once
(`ResolvedBaseImage()`), and feed that resolved string into `renderDockerfile` and
`ConfigHash` exactly where `cfg.Image.BaseImage` goes today. Debian release
(`"trixie"`) becomes a code constant.

**Rationale**: Hash inputs keep identical semantics — same config keeps producing the
same rendered Dockerfile, so FR-008 (version change ⇒ new image, no change ⇒ no
rebuild) holds with zero hash-logic changes.

**Migration note**: A config that previously said `base_image: node:24-trixie` and now
says `node_version: 24` resolves to the identical string — same hash, no rebuild.
Adopting the new default (`lts`) resolves to `node:lts-trixie` — new hash, one rebuild.
Expected and acceptable.

## R7: Error message for confirmed-missing tag

**Decision**: `image.node_version: "<v>" has no published base image (node:<v>-trixie
not found on Docker Hub) — pick a version from https://hub.docker.com/_/node`

**Rationale**: Names the configured value (FR-006a), shows the resolved artifact for
transparency, points at the authoritative tag list.
