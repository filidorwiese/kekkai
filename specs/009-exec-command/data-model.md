# Data Model: kekkai exec

This feature introduces **no new persistent data**: no config keys, no labels, no
files, no volumes. It only reads existing runtime state.

## Entities (existing, read-only)

### Sandbox container

The running container associated with the invoking project directory.

| Attribute | Source | Use by exec |
|---|---|---|
| `kekkai.cwd` label | `internal/runtime/identity.go` (`LabelCwd`) | Resolution key: must equal `$PWD` |
| Running state | `docker.ContainersByLabel` | Only a running container is eligible; none → error |
| Container ID | docker | Target of `docker exec` |

State transitions: none caused by exec. The container is running before, during,
and after (FR-010); exec never changes its lifecycle.

### Exec invocation (transient, in-memory only)

| Attribute | Constraint |
|---|---|
| Command words | ≥ 1 word after stripping one optional leading `--` (else usage error) |
| TTY mode | Derived: stdin is a char device → allocate pty |
| Exit code | Mirrors the executed command's code (contract: [exec-cli.md](contracts/exec-cli.md)) |

Nothing is recorded or persisted about an invocation.
