# Research: Host UID/GID Match

No NEEDS CLARIFICATION markers in the spec. Research resolves the design choices behind the spec's assumptions.

## D1: Where to apply the identity — build time vs run time

**Decision**: Bake the host uid/gid into the image at build time by rendering them into `embed/Dockerfile.tmpl` (`groupadd -g {{.Gid}}` / `useradd -u {{.Uid}}`).

**Rationale**:
- The whole image is already built around a concrete user: nvm and Node install into `/home/kekkai` as that user, `/commandhistory`, `/workspace`, `.bashrc` are chowned at build, sudoers grants by username. Changing identity at run time would require starting as root, `usermod`/`groupmod`, and a recursive re-chown of the home tree on every start.
- Image identity comes free: `ImageTag` is `sha256(rendered Dockerfile + firewall script)` (`internal/runtime/identity.go:58`), so a different uid/gid automatically yields a different tag and triggers a rebuild — FR-003 with zero extra code.
- Images are built per host machine and never distributed (§6), so per-identity images cost nothing.

**Alternatives considered**:
- `docker run --user $(id -u):$(id -g)`: rejected — breaks `/home/kekkai` ownership, nvm paths, and leaves the user with an unknown gid inside; sudoers still matches (uid 1000 → name kekkai) only by accident when uids collide.
- Entrypoint-time `usermod` + `chown -R`: rejected — needs a root entrypoint (larger attack surface vs Principle II), slow start, more moving parts.

## D2: Safe-range gate and fallback

**Decision**: Use the host identity only when `uid >= 1000 && gid >= 1000`; otherwise fall back to the historical 1000/1000. Implemented as one pure helper (`sandboxIdentity()`), unconditional on GOOS.

**Rationale**:
- uid 0 (root) must never become the sandbox user (Principle II).
- Ids below 1000 collide with Debian system accounts/groups in the base image (e.g. gid 20 = `dialout`, gid 100 = `users`); mapping the sandbox user into a system group would grant device/file access the group implies — a silent privilege widening.
- macOS falls out for free: uid 501 / gid 20 are below the gate, so darwin renders the exact same Dockerfile as today → identical image hash → FR-008 (macOS unchanged) holds with no platform branch, honoring Principle III ("never a per-runtime code matrix").

**Alternatives considered**:
- Per-component fallback (keep uid 1000, map only gid): rejected — half-matched identities are harder to reason about and the only real-world beneficiary (Linux gid 100 `users` setups) would gain a system-group membership inside the sandbox.
- Platform branch (skip on darwin): rejected — the range gate already covers it; capability/branching by GOOS violates Principle III.

**Known limitation** (documented in spec assumptions): Linux users with a system-range primary gid (e.g. 100 `users`) keep today's behavior.

## D3: Collision handling inside the Dockerfile

**Decision**:
- Group: `getent group {{.Gid}} >/dev/null || groupadd -g {{.Gid}} kekkai`, then `useradd -g {{.Gid}}` (numeric). If the gid already exists, the sandbox user joins that group; the numeric gid is what matters for host ownership.
- All build-time `chown kekkai:kekkai` become numeric `chown {{.Uid}}:{{.Gid}}` so they work regardless of the group's name.
- Uid: no collision handling — after the >= 1000 gate, the Debian base and all apt system accounts (< 1000 by policy) cannot collide.

**Rationale**: FR-005 (build must not fail on collision) with two shell tokens; numeric chown removes the only name-based assumption.

**Alternatives considered**: `groupadd -f`: rejected — with `-g` taken, `-f` silently creates the group with a *different* gid, defeating the purpose.

## D4: Offline fallback correctness (`ConfigHash`)

**Decision**: Add uid/gid to `ConfigHash` inputs (`internal/runtime/identity.go:67`) and its call site (`internal/runtime/up.go:169`).

**Rationale**: `ConfigHash` keys the §6.2 offline fallback (`newestImageForConfig`). Without identity in it, a network-degraded `up` by user B could reuse an image baked for user A's gid — violating FR-004. `ImageTag` needs no change (rendered Dockerfile already contains the ids).

**Alternatives considered**: Separate identity label on the image: rejected — a second matching key where one hash already exists; ConfigHash is documented as "the bake inputs minus the claude version", and identity is now a bake input.

## D5: Spec-first obligation

**Decision**: Rewrite the §6.3 sandbox-user sentence in `SPECIFICATION.md` (line 186: "UID/GID 1000") to describe host-matched identity with the >= 1000 gate and 1000/1000 fallback, in the same commit as the code (Constitution I).

## D6: Validation approach

**Decision**: End-to-end per constitution IV and the existing e2e notes (pseudo-TTY trick for `up`): build binary, run `up` on this host (uid 1000/gid 1001), `id` inside, create file, `stat` on host, firewall probes pass; fallback case exercised via `sudo ./kekkai up` expectation documented in quickstart (rebuild → 1000/1000). No unit tests beyond `go vet`/build, matching project practice; `sandboxIdentity()` kept pure so it *can* be table-tested if the project ever grows a suite.
