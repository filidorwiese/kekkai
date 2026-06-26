# PRD: Kekkai v0.1.0 (Go rewrite)

## 1. Problem

Kekkai today is a `.devcontainer/` + `Taskfile.yml` setup. To use it from any project folder a user must install three host dependencies (Docker, the `devcontainer` CLI, Task) and must globally register the Taskfile. The firewall allowlist, apt package list, and host mounts are hardcoded across three files; nothing is project-customizable without editing kekkai itself.

This is acceptable for one developer but blocks public distribution and per-project configuration (e.g., a Terraform-heavy project that needs `registry.terraform.io` allowed, an AWS project that needs `~/.aws` mounted in).

## 2. Goals

- One host dependency: **Docker**. (Plus `git` + `curl` for installer; both effectively universal.)
- One install command: `curl -fsSL https://raw.githubusercontent.com/filidorwiese/kekkai/main/install.sh | bash`.
- One run command per project: `kekkai up`.
- YAML config: ships with sane defaults; user overrides at `~/.kekkai.yml`; project overrides at `./.kekkai.yml`.
- Strict containment: container can only egress to an allowlist; only `$PWD` and explicitly listed host paths are mounted; no privileged exec on host.
- Public release: GitHub Releases, semver from `v0.1.0`, prebuilt `linux-amd64` + `linux-arm64`.

## 3. Non-goals (v0.1.0)

- macOS / Windows support.
- Self-updater (`kekkai update`).
- Multiple sandboxes per project folder.
- Per-project Dockerfile override.
- VS Code / devcontainer-CLI integration.
- LICENSE file.

## 4. User stories

- *As a developer*, I want to `cd` into any project and run `kekkai up` to get a sandboxed Claude Code session with no per-project setup.
- *As a developer working on a Terraform project*, I want to drop a `./.kekkai.yml` adding `registry.terraform.io` to allowed domains so Claude can run `terraform init`.
- *As a developer*, I want my Claude session state (`~/.claude`), git config, and SSH agent to be available inside the sandbox without copying secrets in.
- *As a developer*, I want a single `kekkai down` to stop and remove the container; or, more commonly, the container should auto-remove when Claude exits.
- *As a developer with stale projects*, I want `kekkai prune` to clean up containers and images for folders I've deleted.
- *As a new user*, I want `kekkai doctor` to tell me whether my host is set up correctly.

## 5. Locked design decisions

(All decided during grilling; recorded here so future reviewers don't re-litigate.)

- **Language**: Go. stdlib `flag` for subcommands. Shell out to `docker` (no Docker SDK).
- **Distribution**: GitHub Releases, prebuilt `linux-amd64` + `linux-arm64`. `curl|bash` installer downloads to `~/.local/bin/kekkai`. No sudo. Installer prints PATH instructions if needed; does **not** modify shell rc. Installer prints docs URL `https://github.com/filidorwiese/kekkai`.
- **Versioning**: semver from `v0.1.0`. Version string injected via `-ldflags "-X main.version=..."`.
- **Config**: YAML (`gopkg.in/yaml.v3`). Embedded defaults → `~/.kekkai.yml` → `./.kekkai.yml`. **Strict** validation (unknown keys = error via `decoder.KnownFields(true)`). `~` and `${VAR}` expanded at config-load time; unset `${VAR}` errors unless the surrounding object is `optional: true`.
- **Merge**: append-only for arrays except `claude.args` which **replaces**. CLI `kekkai up -- <claude args>` appends to merged `claude.args` at runtime.
- **Container identity**: name `kekkai-<sanitized-basename>-<sha256(PWD)[:8]>`. Authoritative key = label `kekkai.cwd=$PWD`. Additional labels: `kekkai.image_hash`, `kekkai.version`.
- **Image hash**: sha256 over rendered `Dockerfile` + rendered `init-firewall.sh` content. Tag `kekkai:<hash[:12]>`. Build only if `docker image inspect` misses.
- **Lifecycle**: `docker run --rm -it`. Signal forwarding (SIGINT/SIGTERM → child docker → wait → exit). On duplicate `up` for same `$PWD`: refuse with clear message; `--force` recreates.
- **Per-folder bash history**: named volume `kekkai-history-<sha[:8]>`. Survives `prune` unless `--volumes` is passed.
- **`prune`**: removes containers whose `kekkai.cwd` label points to a non-existent host folder; removes `kekkai:*` images not in use; `--volumes` extends to named volumes. Prompts before deletion unless `--yes`.
- **Build progress**: buildkit default (compact streaming). `--verbose` sets `BUILDKIT_PROGRESS=plain`.
- **`WORKSPACE` env var**: auto-injected by kekkai as `basename($PWD)`, not configurable.
- **`claude.args` default**: `"--dangerously-skip-permissions"`. The sandbox itself is the security boundary; the flag is what makes kekkai useful as an unattended Claude environment.

## 6. Tool / image version policy

Defaults below are **starting points**; the implementer should verify against latest stable releases at implementation time and bump as needed.

| Component | Default | Source / rationale |
|---|---|---|
| Base image | `node:22` | Node 22 is active LTS as of June 2026 (Node 24 is "Current" until Oct 2026, then becomes LTS). |
| `git-delta` | verify latest at implementation | Pin to a specific release tag in defaults. |
| `zsh-in-docker` | verify latest at implementation | Pin to a specific release tag in defaults. |
| `tflint` | verify latest at implementation | Pin to a specific release tag in defaults. |
| `@anthropic-ai/claude-code` | `latest` | Floats; rebuild whenever image hash changes due to other config. |

All pinned versions live in `embed/defaults.yml` under `image.*`. Bumping = editing defaults + re-releasing kekkai.

## 7. Subcommands (MVP)

- `up` — load+merge config, render Dockerfile + firewall.conf, compute image hash, build if missing, `docker run --rm -it` with mounts/env/caps. Appends `os.Args` after `--` to `claude.args`. Refuses if a container with matching `kekkai.cwd` label is already running (`--force` to recreate).
- `down` — find container by `kekkai.cwd=$PWD` label, `docker rm -f`.
- `shell` — find container by label, `docker exec -it <id> zsh`.
- `ps` — `docker ps --filter label=kekkai.cwd` with a tidy table.
- `prune` — remove orphan containers + unused `kekkai:*` images. `--volumes` extends. Prompts before deleting unless `--yes`.
- `doctor` — Tier 1+2 fail, Tier 3 warn.
  - **T1 (blocking)**: `docker` binary in PATH, daemon reachable, group membership / sudo-less access; `git` and `curl` in PATH.
  - **T2 (blocking)**: configs at both paths parse; merged config is semantically valid (no duplicate mount targets, etc.).
  - **T3 (warn)**: mount sources exist on host, allowed domains resolve via DNS, `$SSH_AUTH_SOCK` set + socket reachable, installed kekkai version vs latest GH release tag.
- `version` — print embedded version.
- `help` — subcommand listing.

## 8. Config schema

```yaml
image:
  base: node:22
  apt_packages: [less, git, procps, fzf, zsh, man-db, unzip, gnupg2, gh,
                 iptables, ipset, iproute2, dnsutils, aggregate, jq, nano, vim,
                 python3, python3-pip, python3-venv, pipx]
  git_delta_version: "<latest stable>"
  zsh_in_docker_version: "<latest stable>"
  tflint_version: "<latest stable>"

mounts:
  - { source: ~/.claude, target: /home/node/.claude }
  - { source: ~/.gitconfig, target: /home/node/.gitconfig, readonly: true }
  - { source: ~/.config/git/allowed_signers, target: /home/node/.config/git/allowed_signers, readonly: true }
  - { source: ${SSH_AUTH_SOCK}, target: /ssh-agent, optional: true }

env:
  CLAUDE_CONFIG_DIR: /home/node/.claude
  NODE_OPTIONS: "--max-old-space-size=4096"
  POWERLEVEL9K_DISABLE_GITSTATUS: "true"
  SSH_AUTH_SOCK: /ssh-agent

firewall:
  allow_github_meta: true
  allow_host_lan: true
  allowed_domains: [registry.npmjs.org, api.anthropic.com]

caps: [NET_ADMIN, NET_RAW]

claude:
  args: "--dangerously-skip-permissions"
```

`~/.kekkai.yml` and `./.kekkai.yml` use the same schema; missing top-level keys are fine.

## 9. Image build & firewall runtime

- Dockerfile template (`embed/Dockerfile.tmpl`) is rendered with `image.*` values. `apt_packages` becomes a single `RUN apt-get install -y …` line.
- `init-firewall.sh` is baked into the image as a static script. The runtime allowlist is passed in via bind mount — not bake time — so image hash does **not** depend on `firewall.*`.
- At `docker run` time, kekkai renders the merged firewall config to `$TMPDIR/kekkai-<container>/firewall.conf` on the host and bind-mounts it read-only at `/etc/kekkai/firewall.conf`. Format:
  ```
  ALLOW_GITHUB_META=1
  ALLOW_HOST_LAN=1
  ALLOWED_DOMAINS="registry.npmjs.org api.anthropic.com"
  ```
- Container `CMD`: `sudo /usr/local/bin/init-firewall.sh && exec claude $CLAUDE_ARGS`. The sudoers entry that lets `node` run the firewall script is set up at image build (same as today).

## 10. Repo layout (after rewrite)

```
cmd/kekkai/main.go                 # subcommand dispatch
internal/config/load.go            # parse + merge defaults/global/project
internal/config/expand.go          # ~ and ${VAR} expansion
internal/config/validate.go        # strict schema check
internal/docker/exec.go            # exec.Cmd wrappers, signal forwarding
internal/docker/inspect.go         # image/container lookup by label
internal/image/hash.go             # sha256 of rendered Dockerfile+firewall
internal/image/build.go            # render + invoke docker build
internal/firewall/render.go        # render runtime firewall.conf
internal/runtime/up.go
internal/runtime/down.go
internal/runtime/shell.go
internal/runtime/ps.go
internal/runtime/prune.go
internal/runtime/doctor.go
embed/Dockerfile.tmpl              # text/template
embed/init-firewall.sh             # static, reads /etc/kekkai/firewall.conf at runtime
embed/defaults.yml
install.sh                         # curl|bash entry
.github/workflows/release.yml      # matrix build amd64+arm64, publish tar.gz + SHA256SUMS
README.md                          # rewritten
CLAUDE.md                          # rewritten
go.mod / go.sum
PRD.md                             # this document, copied to repo root
```

Deleted at end of rewrite: `.devcontainer/`, `Taskfile.yml`.

## 11. Critical existing files to port

- `.devcontainer/Dockerfile` → `embed/Dockerfile.tmpl`, parameterized over `image.*`. Preserve sudoers grant for `init-firewall.sh`; preserve non-root `node` user; preserve npm-global path; preserve `git-delta` install; preserve `zsh-in-docker` install.
- `.devcontainer/init-firewall.sh` → `embed/init-firewall.sh`. Change: source `/etc/kekkai/firewall.conf` for `$ALLOWED_DOMAINS`, `$ALLOW_GITHUB_META`, `$ALLOW_HOST_LAN` instead of reading sibling `allowed-domains.txt`.
- `.devcontainer/allowed-domains.txt` and the apt list in `Dockerfile` → `embed/defaults.yml` (single source of truth).
- `.devcontainer/devcontainer.json` mounts/env/caps → `embed/defaults.yml`.

## 12. Risks & mitigations

| Risk | Mitigation |
|---|---|
| First `kekkai up` on fresh machine is slow (~3 min image build). | Print "first build takes a few minutes, cached after" before invoking docker build. |
| User edits config, hits typo, gets cryptic error. | Strict YAML validation with line numbers in error messages. |
| `$SSH_AUTH_SOCK` unset breaks default config. | Mount marked `optional: true`; warned in `doctor` T3, silently skipped at runtime. |
| GitHub `/meta` API down → no allowlist for github CIDRs. | Firewall script fails hard rather than fall through; container exits, `kekkai up` reports clearly. (Same behavior as today.) |
| `~/.claude` not present (fresh machine). | Mount creates it; document in README. |
| Container left behind after `kill -9 kekkai`. | `--rm` covers normal exit + SIGINT/SIGTERM; `kekkai prune` covers the rest. |
| Image hash collisions across versions of kekkai itself. | Hash includes rendered template content, so template changes invalidate; kekkai version is also in container label for debuggability. |
| `~/.local/bin` not on user PATH. | Installer prints clear instructions; `kekkai doctor` does not need to live in PATH so chicken-and-egg doesn't bite. |

## 13. Verification (end-to-end)

On a Linux dev box:

1. `go build -ldflags "-X main.version=v0.1.0-dev" -o /tmp/kekkai ./cmd/kekkai`
2. `/tmp/kekkai doctor` — expect green T1+T2, possibly some T3 warnings.
3. `cd ~/some-test-project && /tmp/kekkai up`
   - First run builds image (~3 min), then drops into claude inside the sandbox.
   - Inside container: `echo $WORKSPACE` shows `some-test-project`; `curl https://example.com` fails; `curl https://api.github.com/zen` succeeds; `~/.claude` is read/write; `ssh-add -l` works (if agent forwarded).
4. From a second host terminal: `/tmp/kekkai shell` — opens zsh in the running container. Verify same `kekkai.cwd` label.
5. From a second host terminal: `/tmp/kekkai ps` — lists the running container with cwd column.
6. Add `./.kekkai.yml` with `image: { apt_packages: [htop] }`; exit claude; `/tmp/kekkai up` — rebuilds (new image hash) and `htop` is available inside.
7. Add a typo to `./.kekkai.yml` (e.g. `mountz:`); `/tmp/kekkai up` — errors with unknown key + line number.
8. `/tmp/kekkai down` — container gone. `/tmp/kekkai prune` — orphan image cleaned. `/tmp/kekkai prune --volumes` — history volume cleaned.
9. Ctrl+C during a running `up` — container exits cleanly via `--rm` + signal forwarding; no dangling container in `docker ps -a`.
10. Tag `v0.1.0`, push, GH Actions release runs, install.sh on a fresh Linux VM downloads, places binary in `~/.local/bin`, prints PATH hint + docs URL, `kekkai doctor` passes.

## 14. Open items deferred (do not block v0.1.0)

- `kekkai update` self-updater.
- `kekkai exec` arbitrary command runner (use `kekkai shell` for now).
- `kekkai config show` to dump merged effective config.
- Schema subtraction (e.g. `apt_packages_remove`).
- macOS / Windows.
- Per-project Dockerfile override.
