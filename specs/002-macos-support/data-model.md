# Data Model: macOS Support

No persisted data. Two in-memory types introduced, both host-side, both darwin-scoped.

## RuntimeIdentity (enum)

Detected lazily (only on preflight failure) from `docker info` — see research.md R3.

| Value | Match source | Used for |
|---|---|---|
| `DockerDesktop` | OperatingSystem contains "docker desktop" | file-sharing + agent hints |
| `OrbStack` | OperatingSystem/Name contains "orbstack" | agent hint (native) |
| `Colima` | Name/context "colima" | `colima start --ssh-agent` / mount hints |
| `Unknown` | fallback | generic hint text |

Rules: never gates behavior (spec FR-004); adding a new recognized runtime = new enum value + hint strings only.

## PreflightCheck (probe input/outcome)

One probe container aggregates all checks (research.md R4).

**Inputs** (assembled from validated config + environment):

| Field | Source | Condition |
|---|---|---|
| workspace bind | `$PWD` | always |
| claude-dir bind | `~/.claude` | always |
| gitconfig bind | `~/.gitconfig` | `git.enabled: true` and file exists |
| user mount binds | `disk.mounts[].HostPath` (resolved, non-skipped) | per mount |
| agent socket bind | `/run/host-services/ssh-auth.sock` (VM path) | `git.ssh_agent: true` |
| image | tag from `ensureImage` | always |

**Outcomes** (state transitions):

```text
probe run ──ok──────────────→ proceed to real `docker run`
        └─docker error──────→ FAIL(bind): name offending path, hint per RuntimeIdentity
        └─exit≠0 (test -S)──→ FAIL(agent-socket): name capability, hint per RuntimeIdentity
```

FAIL always aborts before the real sandbox starts; exit code 1; no host artifacts created (probe binds are read-only).

## Config semantics change (existing entity, platform-split)

`git.ssh_agent: true` at `up` time:

| Platform | Requirement | Failure mode |
|---|---|---|
| linux | host `$SSH_AUTH_SOCK` set | hard error (unchanged, §4.4) |
| darwin | VM socket `/run/host-services/ssh-auth.sock` exists | preflight FAIL(agent-socket) |

Validation (`internal/config`) stays platform-neutral; the split lives in `up`/preflight.
