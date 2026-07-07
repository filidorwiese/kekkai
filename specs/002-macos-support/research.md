# Research: macOS Support

All NEEDS CLARIFICATION items from Technical Context resolved. Facts marked *(verify e2e)* must be confirmed on real hardware during implementation — consistent with constitution Principle IV; the capability probe contains the blast radius if a runtime deviates.

## R1. What already works unchanged on macOS

**Decision**: no changes to image pipeline, firewall script mechanics, caps, container lifecycle, labels, volumes.

**Rationale**: every macOS docker runtime runs containers inside a Linux VM; in-container behavior (iptables/ipset, NET_ADMIN/NET_RAW, sudoers grant, verification probes) is identical to Linux. Kekkai shells out to the docker CLI, so daemon/context resolution (Docker Desktop `desktop-linux`, OrbStack, colima contexts) comes free. Named volumes (`kekkai-history-*`), tmpfs shadows, `/dev/null` binds (VM's /dev/null), and `.git` ro binds all operate inside the VM.

**Alternatives considered**: none needed.

## R2. `git.ssh_agent` mechanism on macOS

**Decision**: on darwin, bind `/run/host-services/ssh-auth.sock` (a path inside the runtime's VM) to `/ssh-agent` and set `SSH_AUTH_SOCK=/ssh-agent`, replacing the Linux behavior of binding host `$SSH_AUTH_SOCK`. The linux-only hard error on unset `$SSH_AUTH_SOCK` does not apply on darwin; instead the preflight probe (R4) verifies the VM socket exists.

**Rationale**: a Mac host unix socket cannot cross the VM boundary. Docker Desktop forwards the host agent to `/run/host-services/ssh-auth.sock` inside its VM; OrbStack exposes the same Docker-Desktop-compatible path; colima exposes it only when started with `colima start --ssh-agent` *(verify e2e per runtime; Docker Desktop is the maintainer-validated one)*. One magic path covers all recognized runtimes.

**Alternatives considered**: hard-error always on macOS (spec clarification Q2 rejected it); socat/TCP relay of the agent (new dependency + weaker security, rejected).

## R3. Runtime identity detection (hints only)

**Decision**: `docker info --format '{{.OperatingSystem}}|{{.Name}}'`, matched case-insensitively: contains "docker desktop" → Docker Desktop; contains "orbstack" → OrbStack; name/context "colima" → colima; anything else → Unknown. Executed **only when a preflight probe fails**, to pick the hint text. Never gates (spec clarification Q1).

**Rationale**: identity strings are brittle across versions — acceptable for hint selection (worst case: generic hint), unacceptable for gating. Running it lazily keeps the happy path at zero extra docker calls.

**Alternatives considered**: `docker context show` (misses "default"-context setups); version-string allowlists for gating (rejected by Q1).

## R4. Capability preflight mechanics

**Decision**: darwin-only, after `ensureImage`, before `docker run`: launch **one** short-lived container from the just-built kekkai image, bind-mounting read-only every host path the real run will bind ($PWD, `~/.claude`, `~/.gitconfig` when applicable, each resolved `disk.mounts` source) plus the VM agent socket when `git.ssh_agent: true`; command is `test -S /ssh-agent` when the agent is configured, `true` otherwise. Non-zero exit or docker error → preflight failure: name the capability, add runtime-specific hint (R3), abort before the real sandbox starts.

**Rationale**: one container run (~1-2s) validates all bind-ability (Docker Desktop "mounts denied" for unshared paths, colima unmounted dirs) and socket presence in a single shot, using an image that necessarily exists (just ensured) — no extra pulls. Probing after image build is acceptable: an image build that succeeds but a run that can't mount is exactly the confusing failure this preflight exists to catch early, and config validation (§4.4) still runs first.

**Alternatives considered**: per-mount probe containers (N runs, slow); parsing the real run's failure output (sandbox work already started, violates FR-004's "before any sandbox work" for probes); host-side heuristics like parsing Docker Desktop's settings file (runtime-specific file formats — exactly the coupling Q1 rejected).

## R5. Mac-host reachability (FR-008)

**Decision**: add `add_domain host.docker.internal warn` to the builtin hosts section of `embed/init-firewall.sh`. No env var, no platform branch.

**Rationale**: on macOS runtimes the name resolves (via the runtime's DNS) to the host gateway → Mac host becomes builtin-allowed, matching Linux's builtin bridge-subnet allowance (Q3). On Linux default bridge the name doesn't resolve → existing warn+skip tier, zero behavior change. The script stays byte-identical across platforms, preserving the single-image-hash design (§6.1); the one-time hash change on upgrade is a normal script-edit rebuild.

**Alternatives considered**: `--add-host` + env-injected toggle (more moving parts, runtime config leaking into firewall inputs); resolving on the kekkai side and passing a CIDR (host gateway IP is runtime-internal; DNS at firewall-time is the reliable source). Documented consequence (threat model): unlike Linux, this also reaches Mac services bound to 127.0.0.1.

## R6. Installer + checksum on macOS

**Decision**: `install.sh` accepts `Darwin`: arm64 → `kekkai_${VERSION}_darwin_arm64.tar.gz`; x86_64 under Darwin → explicit refusal ("kekkai supports Apple silicon Macs only"). Checksum verification uses `sha256sum` when present, else `shasum -a 256` (macOS ships shasum, not sha256sum).

**Rationale**: spec clarification Q4 (arm64 only, clear Intel refusal). Rosetta is irrelevant: arm64 Macs run the native binary; Intel Macs can't run arm64.

**Alternatives considered**: Homebrew formula (new distribution surface, deferred); universal binary (needs lipo/CGO toolchain, violates single-static-binary simplicity).

## R7. Release matrix

**Decision**: extend the existing matrix to explicit os/arch pairs: linux/amd64, linux/arm64, darwin/arm64. `CGO_ENABLED=0` cross-compile from ubuntu-latest; tarball naming follows the existing `kekkai_${VERSION}_${GOOS}_${GOARCH}.tar.gz` pattern; same `SHA256SUMS`.

**Rationale**: pure-Go codebase (stdlib + yaml.v3) cross-compiles to darwin without a mac runner; binaries are unsigned/un-notarized — acceptable for a CLI installed via curl/tarball (Gatekeeper quarantine applies to browser downloads, not curl|sh installs; documented if users report issues).

**Alternatives considered**: macOS runner + codesign/notarization (Apple Developer account dependency, real cost; revisit only if quarantine bites in practice).

## R8. What is documented, not engineered around

- Shared-folder I/O (virtiofs et al.) is slower than native Linux binds → README known limitation.
- `host.docker.internal` reaching localhost-bound Mac services (broader than Linux bridge semantics) → threat-model bullet.
- Support tiering: Docker Desktop maintainer-validated per release; OrbStack/colima/others capability-probed, community-validated (Q5) → README.
