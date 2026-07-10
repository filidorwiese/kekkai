# Data Model: Host UID/GID Match

No persisted entities. The feature introduces one derived value and threads it through two existing identity computations.

## SandboxIdentity (derived, per invocation)

| Field | Type | Source | Constraints |
|-------|------|--------|-------------|
| Uid | int | `os.Getuid()` | Rendered value always >= 1000 (gate) |
| Gid | int | `os.Getgid()` | Rendered value always >= 1000 (gate) |

**Derivation rule** (pure function `sandboxIdentity()`):

```text
uid, gid := os.Getuid(), os.Getgid()
if uid < 1000 || gid < 1000 → (1000, 1000)   // root, system ids, macOS 501/20
else → (uid, gid)
```

- No configuration input: identity is never user-configurable (Principle III — no new config key; §6.1 — bake inputs only).
- Not persisted: recomputed on every `up`; the image hash is the only durable record.

## State transitions

None. Identity change between invocations is handled implicitly:

```text
host identity A ──render──> Dockerfile_A ──sha256──> kekkai:hash_A
host identity B ──render──> Dockerfile_B ──sha256──> kekkai:hash_B  (distinct → rebuild)
```

## Modified computations

| Computation | Location | Change |
|-------------|----------|--------|
| Dockerfile render | `internal/runtime/up.go` `renderDockerfile` | Template data struct gains `Uid`, `Gid` ints |
| `ImageTag` | `internal/runtime/identity.go:58` | Unchanged (hash input already contains ids via rendered Dockerfile) |
| `ConfigHash` | `internal/runtime/identity.go:67` | Gains uid/gid inputs, joined into the hashed string; call site `up.go:169` updated |

## Template contract (`embed/Dockerfile.tmpl`)

| Placeholder | Meaning | Invariant |
|-------------|---------|-----------|
| `{{.Uid}}` | Sandbox user numeric uid | >= 1000 |
| `{{.Gid}}` | Sandbox user numeric gid | >= 1000; group may pre-exist in image (reused, not recreated) |

In-image invariants preserved for any rendered identity:
- User name is always `kekkai` (sudoers, `USER kekkai`, `/home/kekkai` paths untouched).
- All build-time chowns are numeric (`{{.Uid}}:{{.Gid}}`), never name-based on the group.
