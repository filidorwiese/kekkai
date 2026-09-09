# Implementation Plan: Inspect model-provider requests (`kekkai mpr`)

**Branch**: `025-mpr-inspect` | **Date**: 2026-09-08 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/025-mpr-inspect/spec.md`

## Summary

Every sandbox gets an always-on, in-sandbox loopback reverse proxy for the model provider: `up` sets builtin env `ANTHROPIC_BASE_URL=http://127.0.0.1:4141` (user `env` overrides it by docker's last-value-wins, which is the whole opt-out), and the image CMD starts an embedded stdlib-Python script (`/usr/local/bin/kekkai-mpr serve`, fixed upstream `https://api.anthropic.com`) under a three-line shell restart loop before `exec claude`. The proxy forwards everything verbatim (auth headers included, never recorded), streams responses through unchanged, and publishes one JSON event per request and per completed response to an abstract unix socket where any number of `follow` subscribers receive them live-only; no subscriber means the event is dropped. Host-side `kekkai mpr [--raw]` mirrors `traffic`: resolve the container by label, check the container env for the bypass/predates cases, `docker exec kekkai-mpr follow`, then reassemble SSE bodies into final message objects and render either a colored full-request transcript with `==== #N REQUEST/RESPONSE` separators or JSON lines. New embedded script and Dockerfile text are bake-time inputs (one-time rebuild); the only firewall touch is `! -o lo` on the two `allow_all` observe taps. Research: [research.md](research.md).

## Technical Context

**Language/Version**: Go (stdlib only) for host CLI; Python 3 stdlib (Debian trixie's 3.13) for the in-sandbox script (`http.server`, `http.client`, `socket`, `json`, `threading`)

**Primary Dependencies**: none new. `python3` already builtin (specs/024); `ca-certificates` for upstream TLS; docker CLI (`exec`, `inspect`) as today

**Storage**: none. Abstract unix socket for the stream; `/dev/shm/kekkai-mpr.seq` (tmpfs, bytes) for the exchange counter

**Testing**: end-to-end per constitution IV — [quickstart.md](quickstart.md), 20 scenarios plus the scenario-0 transport spike; `go build` + `go vet`; `python3 -m py_compile` on the script

**Target Platform**: hosts linux/amd64, linux/arm64, darwin/arm64; the proxy runs inside the linux container on all of them, no platform branch

**Project Type**: CLI tool shelling out to docker; one new subcommand, one embedded script, template + spec amendments

**Performance Goals**: added per-request overhead < 50 ms (SC-002) via per-thread upstream connection reuse; first block visible within 2 s of completion (SC-001); zero memory growth without readers (SC-004)

**Constraints**: no new network destinations, no host port, no sudoers change (Principle II); no config key, `--raw` the only flag (Principle III); rendering never elides (clarified FR-008); credentials never captured (FR-012); fail loud at startup, auto-restart mid-session (FR-014/019); runtime env never enters the image hash

**Scale/Scope**: ~8 files. New: `embed/kekkai-mpr.py`, `internal/runtime/mpr.go`, `internal/runtime/mpr_render.go`, `contracts/`. Modified: `embed/embed.go`, `embed/Dockerfile.tmpl`, `embed/init-firewall.sh` (2 rules), `internal/config/config.go` (constant), `internal/runtime/up.go` (env, build dir, hash), `internal/runtime/identity.go` (hash inputs), `internal/docker/cli.go` (`ContainerEnv`), `cmd/kekkai/main.go` (dispatch, usage), `SPECIFICATION.md`, `README.md`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | Same commit amends `SPECIFICATION.md`: §3 (new `mpr` entry), §5.3 (builtin `ANTHROPIC_BASE_URL`, override semantics), §6.1 (script as hash input), §6.3 (script install, `ENV KEKKAI_MPR_URL`, CMD sequence), §7.2 (CMD), §9 (`allow_all` taps exclude lo). README gains the usage line and a limitations note. | PASS |
| II. Container Is the Security Boundary | Proxy lives inside the container, listens on loopback only, has one hardcoded upstream (`api.anthropic.com`, already the §5.4 builtin), no `CONNECT`/host routing, so it is not a relay to new destinations. No docker socket, no sudo grant, no published port. Firewall rules only gain a loopback exclusion on two non-terminating observe taps; verdicts, policies, §9.6 probes untouched. Plaintext HTTP never leaves the netns. A sandbox process can read the capture socket: that is the agent reading its own outbound data, inside the boundary. Credentials pass through the proxy in memory exactly as they pass through claude, never written. | PASS |
| III. Minimal Surface | One subcommand with one flag, justified: the firewall makes *where* auditable, this makes *what* is sent to the one always-open destination auditable. No config key (`ANTHROPIC_BASE_URL` override reuses `env`), no new apt package, no new binary, one embedded script, all values code constants. | PASS |
| IV. End-to-End Validation | quickstart.md: real-daemon scenarios incl. the transport spike, bypass, restart, concurrency, credential grep, no-persistence, traffic interplay, overhead timing. §9.6 probes unchanged and still run every start. | PASS |
| Constraint: image hash from bake-time inputs only | Script + Dockerfile text enter `ImageTag` and `ConfigHash` (§6.1); the runtime env value never does. One expected rebuild. | PASS |
| Constraint: one config file, strict parse | No schema change. `ANTHROPIC_BASE_URL` intentionally not reserved. | PASS |
| §11 out-of-scope list | Untouched. | PASS |

**Post-design re-check**: unchanged after Phase 1. PASS.

## Project Structure

### Documentation (this feature)

```text
specs/025-mpr-inspect/
├── plan.md              # This file
├── research.md          # Phase 0: R1-R11 decisions
├── data-model.md        # Phase 1: exchange, subscriber, wire events, render model, constants
├── quickstart.md        # Phase 1: e2e validation scenarios
├── contracts/
│   ├── mpr-cli.md       # host command: invocation, stream format, colors, --raw, exit codes
│   └── mpr-wire.md      # in-sandbox script modes, CMD, env, image, event lines, firewall touch
└── tasks.md             # Phase 2 (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/kekkai/main.go             # `mpr` dispatch (flag --raw), usage text
embed/embed.go                 # //go:embed kekkai-mpr.py → assets.MprScript
embed/kekkai-mpr.py            # NEW: serve | wait | follow (stdlib-only reverse proxy + pub/sub)
embed/Dockerfile.tmpl          # COPY script → /usr/local/bin/kekkai-mpr; ENV KEKKAI_MPR_URL; CMD loop+wait
embed/init-firewall.sh         # allow_all NEW taps: add `! -o lo`
internal/config/config.go      # const MprBaseURL = "http://127.0.0.1:4141"
internal/docker/cli.go         # ContainerEnv(id) via docker inspect
internal/runtime/identity.go   # ImageTag/ConfigHash take the mpr script as input
internal/runtime/up.go         # builtin env ANTHROPIC_BASE_URL; write script into build dir; template data
internal/runtime/mpr.go        # NEW: Mpr(raw) — resolve container, env checks, follow reader, signals, exit codes
internal/runtime/mpr_render.go # NEW: SSE reassembly, transcript rendering, separators, paint() color helper
SPECIFICATION.md               # §3, §5.3, §6.1, §6.3, §7.2, §9 amendments
README.md                      # usage line, short "Inspecting Claude's API traffic" note, limitations bullet
```

**Structure Decision**: existing single-binary layout. Sandbox-side logic is an embedded script next to `init-firewall.sh`; host-side logic is a `runtime/<name>.go` pair like `traffic.go`, with rendering split into its own file because it is the bulk of the Go code. Generalizing `Yellow` into a `paint` helper keeps the color convention in one place.

## Implementation Notes (for /speckit-tasks)

1. **Spike first** (quickstart scenario 0): script `serve`+`wait`, Dockerfile wiring, env in `up`; confirm claude answers through the proxy with the user's normal login before writing `follow`, `mpr.go`, or rendering.
2. Script order of work: proxy forward path → streaming pass-through → event publish/pub-sub → `wait` → `follow` with reconnect → seq file → caps.
3. Host order: `ContainerEnv` → `mpr.go` attach/exit paths (copy `traffic.go` structure) → `--raw` output → SSE reassembly → transcript rendering → color.
4. Firewall: only the two `allow_all` NFLOG lines change; re-run the §9.6 probe scenario.
5. Docs last, same commit: SPECIFICATION §3/§5.3/§6.1/§6.3/§7.2/§9, README.

## Complexity Tracking

No constitution violations. Table not needed.

## Notes

- No `update-agent-context` script exists in `.specify/scripts/bash/`, and the repo has no project `CLAUDE.md`; the agent-context step is skipped.
- Git is on `main`; no branch hook is configured. Branch creation is the user's call.
