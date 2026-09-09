# Data Model: Mirror the host project path

## ProjectDir (new helper, `internal/runtime/identity.go`)

| Field | Type | Rule |
|---|---|---|
| path | string | `filepath.EvalSymlinks(os.Getwd())`; absolute, cleaned, no trailing slash |

Single source for: `kekkai.cwd` label, `ContainerName`, `HistoryVolume`, bind source and destination, `-w`, `WORKSPACE` env (basename), all subcommand lookups.

## Path validation (host-side, before any docker call)

| Check | Rejects | Error text |
|---|---|---|
| root | `/` | `project path / cannot be mirrored into the sandbox (root directory)` |
| protected | equal to or ancestor of `/home/kekkai`, `/usr/local/bin`, `/commandhistory` (component-wise) | `project path <p> cannot be mirrored into the sandbox: it would overlay <protected>` |
| charset | contains `:` or any byte < 0x20 | `project path <p> contains ':' or control characters, which the container runtime cannot express as a mount destination; move or rename the project` |

Order: run all three, report every violation (same one-pass convention as §4.4). Exit 1, no docker call.

## Bind table (replaces §5.2 fixed targets)

`P` = ProjectDir.path

| Source | Destination | Mode | Condition |
|---|---|---|---|
| `P` | `P` | rw | always |
| `P/.kekkai.yaml` or staged placeholder | `P/.kekkai.yaml` | ro | always |
| `~/.claude` | `/home/kekkai/.claude` | rw | always (unchanged) |
| volume `kekkai-history-<hash(P)>` | `/commandhistory` | rw | always (unchanged) |
| `P/.git` | `P/.git` | ro | `git.enabled: false` and dir exists |
| `/dev/null` | `P/<hide entry>` | ro | `secrets.hide` file |
| tmpfs | `P/<hide entry>` | — | `secrets.hide` dir |
| user mounts | as validated (§4.3) | per entry | unchanged |

Run args tail: `… → limits → -w P`.

## Image (bake-time, unchanged across projects)

| Item | Before | After |
|---|---|---|
| `mkdir -p … /workspace` + chown | present | removed |
| `WORKDIR` | `/workspace` | `/home/kekkai` |
| hash inputs | template + scripts | same (P never enters) |

## Preflight probe (darwin)

| Bind | Before | After |
|---|---|---|
| project | `P:/kekkai-probe/workspace:ro` | `P:P:ro` |
| others | unchanged | unchanged |

## Subcommand exec

`shell`: `docker exec -it -w P <id> bash`. `exec`: `docker exec -i[-t] -w P <id> <cmd…>`.

## State ownership after the change

| State | Location | Isolation | Host-shared |
|---|---|---|---|
| auto-memory, sessions | `~/.claude/projects/<key(P)>/` | per project | yes |
| trust, MCP, prompt history | `~/.claude/.claude.json` → `projects[P]` | per project | only if host sets `CLAUDE_CONFIG_DIR=~/.claude` |
| permission rules | `P/.claude/settings.local.json` | per project | yes (already) |
| bash history | volume `kekkai-history-<hash(P)>` | per project | n/a |
| old `-workspace` key / `projects["/workspace"]` | `~/.claude/…` | orphaned | untouched |
