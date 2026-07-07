# Contract: macOS Preflight (`kekkai up`, darwin only)

Position in `up` flow: config validation (§4.4) → existing-container check → `ensureImage` → **preflight** → `docker run`. On linux this stage is a no-op.

## Probe invocation

One container, read-only binds of every path the real run will bind, plus the agent socket when configured (see data-model.md PreflightCheck):

```text
docker run --rm \
  -v <PWD>:/kekkai-probe/workspace:ro \
  -v <home>/.claude:/kekkai-probe/claude:ro \
  [-v <home>/.gitconfig:/kekkai-probe/gitconfig:ro]
  [-v <mount-src-N>:/kekkai-probe/m<N>:ro ...]
  [-v /run/host-services/ssh-auth.sock:/ssh-agent]
  <kekkai:image-tag> \
  <"test -S /ssh-agent" | "true">
```

Guarantees:

- No writes: all probe binds read-only; the agent socket bind is inspected (`test -S`), not used.
- No host artifacts: all bound paths stat-checked host-side first (existing §5.2/§8 discipline).
- No extra pulls: uses the image `ensureImage` just guaranteed.
- Zero happy-path docker calls beyond the probe itself; identity detection runs only on failure.

## Failure output contract

Format (stderr, exit 1):

```text
kekkai: preflight failed — <capability>
  <detail line naming the offending path or socket>
  fix: <runtime-specific hint | generic hint>
```

Hint table:

| Failure | DockerDesktop | OrbStack | Colima | Unknown |
|---|---|---|---|---|
| bind (mounts denied / unshared path) | add the folder under Settings → Resources → File Sharing | (should not occur; report) | add the path to colima's mounts (`colima start --mount <path>:w` or edit colima.yaml) | share the folder into your runtime's VM |
| agent-socket missing (`git.ssh_agent`) | enable "Allow SSH agent forwarding" / update Docker Desktop | update OrbStack (forwarding is native) | restart with `colima start --ssh-agent` | expose your SSH agent at /run/host-services/ssh-auth.sock in the VM, or set `git.ssh_agent: false` |

Rules:

- Runtime identity NEVER gates: an Unknown runtime with passing probes proceeds normally (FR-004, edge case "Rancher Desktop").
- `git.ssh_agent: true` never silently degrades (FR-003).
- Message text is part of this contract: SC-004 requires every blocking condition to name its remedy.

## Firewall builtin change (all platforms, one line)

`embed/init-firewall.sh`, builtin hosts section (§5.4/§9.4):

```sh
add_domain host.docker.internal warn
```

- macOS runtimes: resolves → Mac host builtin-allowed (FR-008, Linux bridge parity).
- Linux default bridge: does not resolve → existing warn+skip path; no behavior change.
- Verification probes (§9.6) unchanged and never skipped.
- Consequence: image hash changes once (script is a hash input) → normal rebuild on next `up` after upgrade.
