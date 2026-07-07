# Quickstart: macOS Support — e2e validation guide

Constitution Principle IV: validation is end-to-end against a real daemon. Maintainer-validated tier = Docker Desktop on Apple silicon (spec clarification Q5); scenarios marked [community] cover other runtimes.

## Prerequisites

- Apple silicon Mac, Docker Desktop running (validated tier)
- Go 1.24 for local builds; a test project folder under `~`

```sh
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "-X main.version=v0.0.0-dev" -o /tmp/kekkai ./cmd/kekkai
```

## Scenario 1 — happy path (US1 / SC-001..003)

```sh
cd ~/some-test-project
/tmp/kekkai init
/tmp/kekkai up
```

Expected: image builds, preflight passes silently, firewall log shows all §9.6 probes OK **plus** `host.docker.internal` in the allowed list, Claude Code starts. Then verify lifecycle parity: `shell`, `ps`, `down`, `prune` behave as on Linux.

## Scenario 2 — confinement unchanged (FR-002)

Inside the sandbox (`kekkai shell`):

```sh
curl -sS --max-time 5 https://example.com          # must FAIL (blocked)
curl -sS --max-time 5 https://api.anthropic.com    # must succeed
```

`secrets.hide` file reads empty; unlisted host paths absent.

## Scenario 3 — Mac-host reachability (FR-008)

```sh
python3 -m http.server 8123 --bind 127.0.0.1   # on the Mac, outside sandbox
# inside sandbox:
curl -sS --max-time 5 http://host.docker.internal:8123   # must succeed
```

Confirms Linux-parity builtin host access, including the documented localhost-bound nuance.

## Scenario 4 — ssh_agent capability (FR-003, US2)

With `git.enabled: true`, `git.ssh_agent: true`:

- Docker Desktop with agent forwarding on: `kekkai up` proceeds; inside sandbox `ssh-add -l` lists host keys.
- [community] colima started **without** `--ssh-agent`: `kekkai up` must abort at preflight with the `colima start --ssh-agent` hint, before any sandbox container exists (`docker ps -a` shows none).

## Scenario 5 — unshared path (FR-004, US2)

Remove the project folder's parent from Docker Desktop File Sharing (or use a path outside colima mounts [community]) → `kekkai up` must fail at preflight naming the path and the runtime-specific fix; restore sharing → succeeds.

## Scenario 6 — installer (US3, SC-001)

```sh
curl -fsSL https://raw.githubusercontent.com/filidorwiese/kekkai/main/install.sh | bash
kekkai version
```

On Apple silicon: installs darwin_arm64, checksum verified via shasum. On an Intel Mac (or `uname -m` faked to x86_64): clear refusal, exit 1, nothing installed.

## Scenario 7 — Linux regression gate

On any Linux box: `kekkai up` behaves exactly as before this feature — no preflight container appears, firewall logs `WARNING: failed to resolve host.docker.internal, skipping` (or resolves only if the user configured host-gateway), all probes pass.

## Sign-off checklist per release (macOS-affecting changes)

- [ ] Scenarios 1-7 pass on Docker Desktop / Apple silicon (7 on Linux)
- [ ] Firewall verification probes ran on every `up` (SC-003)
- [ ] Contract error texts match [contracts/preflight.md](contracts/preflight.md) (SC-004)
