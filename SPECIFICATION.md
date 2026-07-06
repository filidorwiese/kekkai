# Kekkai — Specification

> Living document. Source of truth for kekkai's design. Update this first when the design changes; code follows spec.

## 1. Purpose

Kekkai runs Claude Code inside a per-project Docker sandbox with explicit control over disk, network, and secrets. The container is the security boundary, so Claude runs with `--dangerously-skip-permissions` — fully autonomous, nothing escapes.

Distribution infrastructure, not application code. One static Go binary (`./cmd/kekkai`), Linux amd64/arm64 only. No unit-test suite of consequence; validation is end-to-end against a real docker daemon:

```sh
go build -ldflags "-X main.version=v0.0.0-dev" -o /tmp/kekkai ./cmd/kekkai
cd ~/some-test-project && /tmp/kekkai up
```

## 2. Threat model

Protects against a misbehaving agent: prompt injection, malicious dependencies, destructive commands. Known limits (documented, accepted):

- Claude Code credentials must live inside the sandbox.
- Any allowed network destination is a potential exfiltration channel — allow sparingly. DNS lookups are a side channel; the firewall constrains connections, not queries.
- `secrets.hide` is an explicit exact-path list; everything else in exposed folders is readable.
- `~/.claude` is mounted read-write so sessions/skills persist — a compromised agent could alter hooks/skills executed later outside the sandbox.
- `git.ssh_agent: true` lets the agent authenticate as the user against any allowed host.
- The docker bridge subnet is always allowed: host services on `0.0.0.0`/bridge IP and neighbor containers on the same bridge are reachable from the sandbox.
- Docker is the boundary; kernel-level container escapes out of scope.
- Docker-in-sandbox is **not supported**: socket access would bypass the sandbox entirely. No escape hatch exists.

## 3. CLI

```
kekkai init        # write starter .kekkai.yaml (errors if one exists)
kekkai up          # build image if needed, start sandbox, exec claude
kekkai down        # stop + remove the sandbox container for $PWD
kekkai shell       # open zsh in the running sandbox for $PWD
kekkai ps          # list running kekkai containers
kekkai prune       # remove orphan containers + unused kekkai:* images; --volumes adds history vols; --yes skips prompt
kekkai version     # print version
kekkai help        # usage
```

- Stdlib `flag` per subcommand, dispatch in `cmd/kekkai/main.go`, logic in `internal/runtime/<name>.go`.
- `up` flags: `--force` (recreate existing container), `--verbose` (plain buildkit progress). Args after `--` are appended to claude args.
- No `config` or `doctor` subcommands.

## 4. Configuration

### 4.1 Model

Single file: `./.kekkai.{yml,yaml}` at the workspace root. Both extensions accepted; both present is an error. **Required** — `up` without it errors: "no .kekkai.yaml found, run `kekkai init`". There are no merged yaml layers and no user-global config; all defaults are code-level constants (§5).

Strict parsing (`yaml.v3`, `KnownFields(true)`). Known keys from the pre-rewrite schema (`image.base`, `image.claude_code_version`, `firewall`, `docker_access`, top-level `mounts`) produce a targeted error: schema changed, run `kekkai init`, see README.

### 4.2 Schema

```yaml
image:
  base_image: node:22-bookworm   # required; must match node:*
  apt_packages: [golang]         # appended to builtin set (§5.1)

claude:
  version: latest                # "latest" (default) or exact e.g. "2.0.14"
  args: "--dangerously-skip-permissions"   # replaces default verbatim

# Sections below are disabled when omitted

git:
  enabled: true                  # true: ~/.gitconfig ro, commits work; false/omitted: .git bound readonly
  ssh_agent: false               # mounts $SSH_AUTH_SOCK + allowed_signers

disk:
  mounts:
    - source: ~/.aws             # target optional (§4.3)
      target: /home/kekkai/.aws
      readonly: true
      optional: true             # skip silently if source missing

env:                             # map, not list
  NODE_ENV: development
  GH_TOKEN: ${GH_TOKEN}          # passthrough pattern for gh auth

network:
  allow_all: false               # escape hatch: true disables the egress firewall; exclusive of all other network keys
  allow_github: false            # git/api/ssh via api.github.com/meta CIDRs
  allowed_cidrs:                 # literal IP ranges, e.g. LAN or staging
    - 192.168.1.0/24
  allowed_domains:               # resolved to IPs once at startup
    - registry.npmjs.org

secrets:
  hide:                          # exact file/dir paths relative to workspace root
    - .env.production
    - deploy/certs               # directories shadowed too (empty tmpfs)

limits:                          # optional; unlimited when omitted
  cpus: 4
  memory: 8g
```

### 4.3 Semantics

- **Mount target inference**: omitted target — `~/x` → `/home/kekkai/x`; absolute non-home source → same absolute path in container. Explicit target always wins.
- **Expansion**: `~` and `${VAR}` expand in mount sources and env values. Unset `${VAR}` errors unless the surrounding mount has `optional: true`.
- **`claude.args` replaces** the default string entirely (keep the flag if autonomous mode wanted).
- **`env` reserved keys** (rejected): `WORKSPACE`, `ALLOW_ALL`, `ALLOW_GITHUB`, `ALLOWED_DOMAINS`, `ALLOWED_CIDRS`, `SSH_AUTH_SOCK` (managed by `git.ssh_agent`).

### 4.4 Validation (fail early)

`kekkai up` validates the full document as its first step — strict schema (unknown keys/types via `KnownFields`) plus all semantic checks below — and aborts before any docker work (version resolution, image build, container run). Report all violations in one pass, not first-error-only.

- `image.base_image` required, must match `node:*` — error otherwise.
- `claude.version`: "latest" or exact npm version string.
- Mounts: source required, no duplicate targets.
- `git.ssh_agent: true` requires `git.enabled: true` — validation error otherwise. At `up`, `ssh_agent: true` with unset `$SSH_AUTH_SOCK` on host is a hard error (no silent skip).
- `network.allow_all: true` combined with any other `network` key — validation error (contradictory intent; the escape hatch must be deliberate and alone). Omitted `network` block = firewall on, builtins only.
- `network.allowed_cidrs`: each entry must parse as CIDR.
- `network.allowed_domains`: no whitespace.
- `limits.cpus` positive number; `limits.memory` must match `^[0-9]+(\.[0-9]+)?[bkmg]?$` (case-insensitive — docker's `--memory` grammar, checked here to keep failures pre-docker).

### 4.5 `kekkai init`

Writes a minimal starter: active keys `image.base_image` + `claude`, all optional sections present but commented out, with README-grade explanatory comments. Includes the commented `GH_TOKEN: ${GH_TOKEN}` env example next to `allow_github` — env passthrough is the supported gh auth path (host keyring tokens don't carry into containers; `gh` reads `GH_TOKEN` before `~/.config/gh/hosts.yml`).

Copy/paste safety: active example values in the starter (and README example) must equal the defaults — pasting unchanged reproduces default behavior. Behavior-changing options (e.g. `--model` in `claude.args`) appear only in comments.

## 5. Builtins (code constants, not user-visible config)

### 5.1 apt packages

Baked into the Dockerfile template, user `apt_packages` appends only.

Required — firewall/lifecycle: `sudo`, `iptables`, `ipset`, `iproute2`, `dnsutils`, `curl`, `ca-certificates`, `jq`, `aggregate`. Required — subcommands: `zsh` (`kekkai shell`). Convenience: `git`, `gh`, `less`, `nano`, `procps`.

`jq`/`aggregate` only exercised on the `allow_github` path but stay baked: the image must be identical regardless of runtime config.

### 5.2 Mounts

- `$PWD` → `/workspace` (rw) — the only writable host bind besides `~/.claude`.
- `~/.claude` → `/home/kekkai/.claude` (rw) — sessions/skills/hooks carry over. Always on, not configurable.
- History volume `kekkai-history-<sha256($PWD)[:8]>` → `/commandhistory`.
- When `git.enabled: true`: `~/.gitconfig` → ro; agent commits carry the user's identity.
- When `git.enabled: false` or section omitted: `$PWD/.git` → `/workspace/.git` ro bind (skipped if not a git repo). Enforceable no-commit: the container lacks `SYS_ADMIN`, so the agent cannot remount. History readable (`log`/`diff`/`show`); commits, staging, fetch, and index refresh fail. Chosen over `chmod -x git` (trivially bypassed by copying the binary, and kills read ops Claude needs).
- When `git.ssh_agent`: `$SSH_AUTH_SOCK` → `/ssh-agent` + env `SSH_AUTH_SOCK=/ssh-agent`; `~/.config/git/allowed_signers` → ro, optional.

### 5.3 Env

`CLAUDE_CONFIG_DIR=/home/kekkai/.claude`, `NODE_OPTIONS=--max-old-space-size=4096`, `POWERLEVEL9K_DISABLE_GITSTATUS=true`, `WORKSPACE=<basename $PWD>`. User env applied before firewall env so firewall vars stay authoritative.

### 5.4 Always-allowed network destinations

`api.anthropic.com`, `statsig.anthropic.com`. Nothing else — not npm, not sentry. Baked into the firewall script, not user-removable, not listed in user config.

## 6. Image

### 6.1 Bake-time inputs (and nothing else)

Dockerfile template (`embed/Dockerfile.tmpl`) rendered with: `image.base_image`, builtin + user `apt_packages`, resolved `claude.version`. Image hash = `sha256(rendered Dockerfile + embed/init-firewall.sh)[:12]`, tag `kekkai:<hash>`. Built on demand only when `docker image inspect` misses.

Each image additionally carries label `kekkai.config_hash = sha256(image.base_image + "\n" + builtin+user apt_packages + "\n" + embed/init-firewall.sh)[:12]` — the bake inputs *minus* the claude version. It exists solely so the §6.2 offline fallback can find images built for this config; it never keys builds.

Everything else (mounts, env, network, secrets, limits, claude.args) is runtime input — must never trigger a rebuild, must never enter the hash.

### 6.2 `claude.version: latest` resolution

At `up`, "latest" is resolved to the concrete current version via the npm registry **before** rendering, so the hash tracks Claude releases and a new release triggers a rebuild. Offline/registry-failure fallback: reuse the newest (by creation date) existing `kekkai:*` image whose `kekkai.config_hash` label matches the current config (§6.1), with a warning; none matching → hard error. Pinned versions render as-is.

### 6.3 Dockerfile contract

- Base `node:*`; `node` user renamed to `kekkai` (UID preserved), home `/home/kekkai`.
- npm global prefix `/usr/local/share/npm-global`, claude installed there.
- zsh history wired to `/commandhistory/.zsh_history`.
- `init-firewall.sh` copied to `/usr/local/bin/`; the **only** sudoers grant: `kekkai ALL=(root) NOPASSWD: /usr/local/bin/init-firewall.sh`. No other sudo without strong reason.
- No docker CLI.

## 7. Runtime

### 7.1 Container identity

- Name: `kekkai-<sanitized-basename($PWD)>-<sha256($PWD)[:8]>`. Sanitized = basename lowercased, every char outside `[a-z0-9_.-]` replaced with `-` (docker name charset; the `kekkai-` prefix guarantees a valid leading char).
- Authoritative key: label `kekkai.cwd=$PWD` — `down`/`shell`/`prune` resolve by label, never by name. Also labels `kekkai.image_hash`, `kekkai.version`. Identity logic lives in `internal/runtime/identity.go`; any change updates every consumer.

### 7.2 Lifecycle

`docker run --rm -it`; removed on claude exit, SIGINT, SIGTERM (signals forwarded by `internal/docker/exec.go`). Existing container for same `kekkai.cwd` → `up` refuses unless `--force`. CMD: `sudo /usr/local/bin/init-firewall.sh && exec claude $CLAUDE_ARGS`.

### 7.3 Run args assembly

`--cap-add NET_ADMIN --cap-add NET_RAW` (required by firewall, not configurable) → builtin mounts → git mounts → disk.mounts (missing source: skip+notice if optional, warn otherwise) → secrets shadows (§8) → builtin env → user env → firewall env (last, authoritative) → `CLAUDE_ARGS` → `limits` (`--cpus`, `--memory`) → `-w /workspace`.

## 8. Secrets hiding

`secrets.hide` paths (relative to workspace root) are shadowed, stat-gated on the host **before** run — docker must never create host artifacts for missing paths:

- Regular file → bind `/dev/null:<path>:ro` (reads empty).
- Directory → tmpfs mount at path.
- Missing → warn + skip.

Exact paths only; no globs. Documented limit: files created later at other paths aren't covered.

## 9. Firewall (`embed/init-firewall.sh`)

Runs as root via the single sudoers grant, before claude starts. Inputs via env injected by `kekkai up` (never via bind-mounted files — host-path binds unreliable across snap/SELinux/rootless/remote daemons): `ALLOW_ALL`, `ALLOW_GITHUB`, `ALLOWED_DOMAINS`, `ALLOWED_CIDRS`.

`ALLOW_ALL=1` (from `network.allow_all`): the script applies no restrictions and skips verification, printing a prominent "egress firewall disabled" warning. Everything else:

1. Flush tables, preserve/restore Docker's embedded-DNS NAT rules.
2. Allow loopback, DNS (udp 53), established/related. **No blanket port allowances** — specifically no global tcp/22 (the ipset match covers all ports to allowed IPs; ssh works to allowed destinations only).
3. Always allow the docker bridge subnet, read from the container's own interface route — host reachability is builtin, not configurable. (The host's physical LAN is *not* reachable this way; container routes only see the bridge. LAN access = user adds the CIDR to `allowed_cidrs`.)
4. Build `allowed-domains` ipset: builtin hosts (§5.4, resolved via dig) + `ALLOWED_DOMAINS` (dig, once, warn+skip on resolution failure) + `ALLOWED_CIDRS` (validated literals) + when `ALLOW_GITHUB=1`, CIDRs from `api.github.com/meta` (jq-validated, aggregated; fetch failure fatal only when github on — fetch happens pre-lockdown).
5. Default policy DROP in/out/forward; allowed-set egress ACCEPT; reject rest with icmp-admin-prohibited.
6. **Verification (never disable):** `https://example.com` must FAIL; `https://api.anthropic.com` must SUCCEED; when `ALLOW_GITHUB=1`, `https://api.github.com/zen` must SUCCEED. Any probe violation aborts startup.

To allow a new destination: user config `network.*` — never by relaxing the script.

## 10. Distribution

- `v*` tag → `.github/workflows/release.yml`: matrix build linux/amd64 + arm64, tarballs, `SHA256SUMS`, GitHub release.
- `install.sh` (curl-pipe from repo main): reads latest tag from GH API (or `KEKKAI_VERSION`), installs to `~/.local/bin/`.
- Host prerequisites: Docker, git, curl.

## 11. Out of scope (do not add without discussion)

- Docker socket in sandbox / docker-in-docker — rejected by threat model, not just deferred.
- macOS / Windows builds. macOS findings (2026-07): the code delta is small — containers run in a Linux VM, so image/firewall/caps work unchanged, and kekkai shells out to the docker CLI so daemon resolution comes free; darwin build + install.sh arch detection is trivial. Rejected because the support surface isn't small: `git.ssh_agent` breaks (unix socket can't cross the VM boundary; Docker Desktop needs the magic `/run/host-services/ssh-auth.sock`, OrbStack/colima differ), the always-allowed bridge subnet reaches the VM rather than the Mac host (host services live at `host.docker.internal`), binds only work under shared paths with slow virtiofs I/O, and each docker runtime (Docker Desktop/OrbStack/colima) needs its own testing. If ever revisited: darwin/arm64 + Docker Desktop only, `ssh_agent` hard-erroring with a clear message.
- `kekkai update` self-updater.
- VS Code / devcontainer-CLI integration.
- Multiple sandboxes per folder.
- Per-project Dockerfile override.
- User-global config layer.

## 12. Document conventions

- This file is the initializing specification and stays authoritative: design changes land here first (same commit as the code), README stays the user-facing digest of this spec.
- Record rejected alternatives inline only when the rejection is load-bearing (e.g. §9 env-vs-bindmount, §11 docker socket).
