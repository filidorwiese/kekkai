# Research: kekkai exec

No NEEDS CLARIFICATION markers remained in the spec. Research resolved the four
behavioral unknowns below by reading the existing code (`internal/runtime/shell.go`,
`internal/docker/exec.go`, `internal/runtime/up.go`) and the docker CLI's documented
`exec` semantics.

## R1 — TTY allocation

**Decision**: Pass `-i` always; pass `-t` only when stdin is a terminal, detected
with `golang.org/x/term.IsTerminal(os.Stdin.Fd())`.

**Rationale**: `docker exec -it` hard-fails with "the input device is not a TTY"
when stdin is piped, which would break the P2 script/pipeline story. `-i` alone
keeps stdin forwarded in both modes. Gating on stdin (not stdout) matches the
convention used by kubectl/docker compose: interactive programs get a pty when the
user is at a terminal; pipelines get plain streams.

**Alternatives considered**: always `-it` (breaks pipes — rejected); never `-t`
(breaks interactive programs like `top`, and ^C would not reach the in-container
process — rejected); a `--tty` flag (violates FR-004's "no exec flags" and minimal
surface — rejected); reusing `up.go`'s `os.ModeCharDevice` stat check (**rejected
after e2e failure**: `/dev/null` is a char device but not a TTY, so `cmd </dev/null`
— common in scripts/CI — passed `-t` and docker refused with "the input device is
not a TTY"; a real isatty is required, and stdlib has none without build-tagged
raw ioctls, so the official `golang.org/x/term` module is the minimal correct
dependency — justified against constitution III in plan.md).

## R2 — Exit-code passthrough

**Decision**: Reuse `docker.Interactive`, which already returns the docker CLI
child's exit code; `docker exec` itself exits with the executed command's code.

**Rationale**: Zero new code satisfies FR-002. Docker's reserved codes surface
naturally: 125 (daemon/exec setup error), 126 (found, not executable), 127 (not
found) — the spec's "command does not exist" edge case flows through as 127, same
as a local shell. Kekkai's own failures (no sandbox, usage error) return 1 through
the existing dispatch error path, distinct in kind (message on stderr, nothing
executed).

**Alternatives considered**: inspecting `docker inspect --format {{.State.ExitCode}}`
after a detached exec — more moving parts, loses live streaming — rejected.

## R3 — Signal behavior (Ctrl+C)

**Decision**: Keep `docker.Interactive`'s existing SIGINT/SIGTERM forwarding to the
docker CLI child, and document the non-TTY residual limitation in the contract.

**Rationale**: With `-t`, the pty delivers ^C directly to the in-container process:
FR-008's interactive case works fully (command dies, nonzero exit, sandbox keeps
running). Without a TTY, the docker CLI exits on the forwarded signal (caller gets
a nonzero exit and control back) but the in-container process may keep running —
a long-standing `docker exec` limitation (no signal proxying to exec'd processes,
moby/moby#9098). The sandbox container itself is never affected.

**Alternatives considered**: tracking the exec PID inside the container and
`docker exec ... kill` on signal — extra plumbing and races for a corner case the
docker CLI itself does not handle; violates minimal surface — rejected.

## R4 — Argument handling and `--`

**Decision**: No `flag.FlagSet` for `exec`. Strip a single optional leading `--`,
require at least one remaining word, pass everything verbatim after the container
ID in the `docker exec` argv.

**Rationale**: FR-004 requires verbatim passthrough. The docker CLI parses `exec`
with interspersed flags disabled: the first argument after its own flags is the
container, everything after is the command — so user words beginning with `-`
(e.g. `kekkai exec ls -la`, even a command literally named `-e`) can never be
swallowed by docker, because kekkai always places the container ID first.
Accepting an optional `--` mirrors `kekkai up`'s separator convention for users
who expect it.

**Alternatives considered**: requiring `--` always (needless friction, no ambiguity
exists — rejected); parsing exec-specific flags before `--` (nothing to configure —
rejected).

## R5 — Container resolution

**Decision**: Identical to `Shell()`: resolve by label `kekkai.cwd=$PWD` via
`docker.ContainersByLabel`, use the first running match, otherwise error
`no running sandbox for <pwd>, run 'kekkai up'` (exit 1). Never create, restart,
or remove containers (FR-010).

**Rationale**: Label resolution is the authoritative identity mechanism (§7.1,
`internal/runtime/identity.go`); reusing the shell error string keeps the CLI
voice consistent and satisfies SC-004.

**Alternatives considered**: auto-starting the sandbox when absent — blurs `up`'s
role, surprising side effects from a "run one command" verb — rejected (spec
assumption).
