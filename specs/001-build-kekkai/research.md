# Research: Build Kekkai

**Feature**: 001-build-kekkai | **Date**: 2026-07-06

No NEEDS CLARIFICATION items remained in Technical Context — SPECIFICATION.md pins every
design-level decision. This document records the decisions with rationale and rejected
alternatives so downstream phases don't re-litigate them.

## R1. Docker integration: shell out to docker CLI

- **Decision**: invoke the `docker` binary (`internal/docker/cli.go`); no Docker SDK.
- **Rationale**: daemon discovery (contexts, rootless, remote, snap) comes free with the CLI;
  interactive TTY + signal semantics of `docker run --rm -it` are exactly what §7.2 needs;
  keeps the module dependency-free beyond yaml. SPECIFICATION.md §11 already assumes this
  ("kekkai shells out to the docker CLI").
- **Alternatives**: `github.com/docker/docker` client SDK — rejected: huge dependency tree
  (violates constitution III), must reimplement context resolution and TTY handling.

## R2. Config parsing: `gopkg.in/yaml.v3` with `KnownFields(true)`

- **Decision**: sole external dependency; decode into typed structs, strict unknown-key
  rejection; a pre-pass detects legacy pre-rewrite keys for the targeted migration error (§4.1).
- **Rationale**: §4.1 mandates strict parsing; yaml.v3 is the only maintained strict-mode YAML
  lib; stdlib has no YAML.
- **Alternatives**: `sigs.k8s.io/yaml` (JSON round-trip loses strictness granularity),
  `goccy/go-yaml` (larger, no benefit here) — rejected.

## R3. Multi-error validation report

- **Decision**: validators append to an error slice; `up` prints all violations then exits
  non-zero (§4.4). Semantic checks (CIDR parse via stdlib `net/netip`, memory string via
  regex-level check, reserved env keys, mount duplicate targets, ssh_agent⇒git.enabled,
  allow_all exclusivity) run after schema decode.
- **Rationale**: spec requires "all violations in one pass, not first-error-only".
- **Alternatives**: fail-fast per error — explicitly ruled out by §4.4.

## R4. Image identity: hash of rendered inputs

- **Decision**: tag = `kekkai:` + `sha256(rendered Dockerfile + init-firewall.sh)[:12]` (§6.1);
  build only when `docker image inspect` misses; runtime config never enters the hash.
- **Rationale**: content-addressing makes rebuild-on-change and reuse-on-same automatic; firewall
  script is baked into the image so it belongs in the hash.
- **Alternatives**: version-string tags (drift risk, manual bumps), always-build with docker
  cache (slow warm path, §SC-001) — rejected.

## R5. `claude.version: latest` resolution

- **Decision**: GET `https://registry.npmjs.org/@anthropic-ai/claude-code/latest` (small JSON,
  read `.version`) before rendering. On failure: reuse the newest existing `kekkai:*` image whose
  `kekkai.config_hash` label (version-independent bake-input hash, §6.1) matches the current
  config, with a warning; no match → hard error (§6.2). The label makes "for this config"
  resolvable when the full hash can't be computed (version unknown).
- **Rationale**: hash must track Claude releases; the `/latest` endpoint returns a tiny document
  vs the multi-MB full packument.
- **Alternatives**: `npm view` on host (requires node on host — new prerequisite, rejected);
  resolving inside the build (hash couldn't see the version, breaking rebuild-on-release).

## R6. Firewall input channel: environment variables

- **Decision**: `ALLOW_ALL`, `ALLOW_GITHUB`, `ALLOWED_DOMAINS`, `ALLOWED_CIDRS` injected as env
  by `up`; script reads only env (§9).
- **Rationale**: host-path bind mounts are unreliable across snap/SELinux/rootless/remote
  daemons (load-bearing rejection recorded in spec §9/§12).
- **Alternatives**: bind-mounted config file — rejected per above; baking rules into image —
  violates §6.1 (runtime input must not trigger rebuilds).

## R7. Enforceable no-commit git mode

- **Decision**: `git.enabled` false/omitted → bind `$PWD/.git` read-only over the workspace
  mount; container has no `SYS_ADMIN` so remount is impossible (§5.2).
- **Rationale**: filesystem-level enforcement survives any in-container trickery; history stays
  readable.
- **Alternatives**: `chmod -x git` — trivially bypassed by copying the binary and kills read ops
  (rejection recorded in spec §5.2).

## R8. Embedded assets

- **Decision**: `go:embed` for `embed/Dockerfile.tmpl` + `embed/init-firewall.sh`; template
  rendered with `text/template`.
- **Rationale**: single static binary is the distribution unit (§1); no runtime file lookups.
- **Alternatives**: installing assets beside the binary — breaks curl-pipe single-file install.

## R9. Signal handling & lifecycle

- **Decision**: `docker run --rm -it` as a child process with stdio attached;
  `internal/docker/exec.go` forwards SIGINT/SIGTERM; `--rm` guarantees removal on any exit
  (§7.2). CMD chain: `sudo /usr/local/bin/init-firewall.sh && exec claude $CLAUDE_ARGS` — probe
  failure aborts before claude starts.
- **Alternatives**: detached container + `docker attach` (orphan risk on crash), managing
  containers via API events — needless complexity.

## R10. Identity derivation

- **Decision**: one function set in `internal/runtime/identity.go`: name
  `kekkai-<sanitized-basename>-<sha256(PWD)[:8]>`, label `kekkai.cwd=$PWD` (authoritative),
  history volume `kekkai-history-<sha256(PWD)[:8]>`. All consumers (`up/down/shell/ps/prune`)
  import it (§7.1).
- **Rationale**: label-keyed resolution survives name collisions/renames; single source
  prevents drift between subcommands.

## R11. Release pipeline

- **Decision**: GitHub Actions on `v*` tags: matrix `GOOS=linux GOARCH={amd64,arm64}`,
  `-ldflags "-X main.version=<tag>"`, tar.gz per arch, `SHA256SUMS`, `gh release create`.
  `install.sh` resolves latest via GH API (override `KEKKAI_VERSION`), installs to
  `~/.local/bin/` (§10).
- **Alternatives**: goreleaser — capable but an extra tool+config for a 2-target matrix;
  rejected under constitution III.
