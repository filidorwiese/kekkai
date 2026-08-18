# Data Model: OpenSSH Client in Sandbox Image

No new entities, config keys, or persisted data. The feature modifies one existing value set:

## Builtin apt package set

- **Location**: `builtinAptPackages` in `internal/runtime/up.go`; documented in SPECIFICATION.md §5.1.
- **Semantics**: bake-time input; concatenated with user `image.apt_packages` (user appends only), rendered into `embed/Dockerfile.tmpl`, feeds image hash (§6.1) and `kekkai.config_hash` label (§6.2).
- **Change**: add `openssh-client`.
- **Invariants preserved**:
  - Set is constant per binary release — never derived from runtime config.
  - User list appends after builtin; duplicates harmless at apt level.
  - Any set change ⇒ new image hash ⇒ one natural rebuild on next `up`.

## Unchanged

- `git.ssh_agent` config key: semantics untouched (mount + env only, §5.2).
- Firewall destination set, sudoers grants, mounts: untouched.
