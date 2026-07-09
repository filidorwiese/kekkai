# Contract: Sandbox image (Dockerfile) — Debian + nvm

**Feature**: 014-debian-nvm-base

Replaces the §6.3 node:*-base contract. Rendered from `embed/Dockerfile.tmpl`; every item below is a bake-time property.

## Base and user

- `FROM debian:trixie` — Debian release is a kekkai code constant, never user input.
- User `kekkai` created with UID 1000 / GID 1000, home `/home/kekkai`, shell `/bin/zsh` (identical numeric identity to the old renamed `node` user; FR-007).
- Baseline apt set = existing builtins + `bash`; GitHub CLI apt-repo bootstrap unchanged.

## Node install (as `kekkai`, never root)

- nvm installed from release tag `v0.40.5` (code constant): installer fetched from the nvm repo at that tag, `NVM_DIR=/home/kekkai/.nvm`.
- Build steps invoke nvm via bash with `BASH_ENV` sourcing `nvm.sh` (`SHELL ["/bin/bash", "-c"]`) — the installer's documented non-interactive pattern (FR-005).
- `nvm install <arg>` + `nvm alias default <arg>` where `<arg>` is per [node-version.md](node-version.md).
- Install failure wraps to: `ERROR: Node version <selector> not found - check image.node_version in .kekkai.yaml` (build aborts; error names the config knob).
- Stable path: `/home/kekkai/.nvm/current` → resolved `versions/node/vX.Y.Z` dir.
- **No `NPM_CONFIG_PREFIX`**: removed (nvm-incompatible). npm globals live in the nvm version dir, owned by `kekkai` → runtime `npm install -g` needs no sudo, creates no root-owned files under `/home/kekkai` (FR-008).

## Exec-path guarantee (FR-005/FR-006)

- Root-created symlinks: `/usr/local/bin/{node,npm,npx,claude}` → `/home/kekkai/.nvm/current/bin/<name>`.
- `ENV PATH=/home/kekkai/.nvm/current/bin:$PATH` — covers binaries added later by runtime `npm install -g`, and outranks any Debian `nodejs` from user `apt_packages`.
- MUST hold: `node -v`, `npm -v`, `npx -v`, `claude --version` succeed from interactive zsh, `docker exec <ctr> node -v`, and `docker exec <ctr> sh -c 'node -v'`, all reporting the same node version.

## Claude Code

- `npm install -g @anthropic-ai/claude-code@<resolved claude.version>` as `kekkai`, against the nvm npm.

## Startup line (FR-012 / SC-007)

- CMD prints, before firewall init, on every container start:
  `kekkai sandbox: node <x.y.z>, claude <resolved version>` (node version read live via `node -v`; claude version rendered at build).
- Firewall init and claude exec behavior after that line: unchanged from §6.3/§7.2.

## Unchanged invariants

- zsh history wired to `/commandhistory/.zsh_history`.
- `init-firewall.sh` copy + the single sudoers grant + `env_keep` line: byte-identical contract to §6.3.
- No docker CLI in the image.
- All existing mount targets under `/home/kekkai/...` (`.claude`, `.gitconfig`, `.config/git/allowed_signers`, user mounts) valid unchanged (FR-009).

## Build-time network (FR-010)

- Downloads during `docker build` (nvm installer/repo, Node tarballs, npm registry) use the docker daemon/host network; the egress firewall does not exist at build time.
- These domains MUST NOT be required in `network.allowed_domains` and MUST NOT be added to the §5.4 builtin set. A Dockerfile comment states this.
