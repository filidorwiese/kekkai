# Kekkai

![Kekkai mascot](kekkai-mascot.png)

> *Kekkai* (結界): a barrier/ward in Japanese folklore that confines spirits within a defined space.

A portable, sandboxed Claude Code container you can run against any project folder.

## What it does

- Builds a `node:22`-based image with Claude Code, zsh, git-delta, lint tools.
- Runs as non-root `kekkai` user (home `/home/kekkai`).
- Applies a strict iptables/ipset egress firewall: only an allowlist of domains plus GitHub meta CIDRs and the host LAN can leave the sandbox.
- Mounts your host `~/.claude` (R/W), `~/.gitconfig` (RO), git allowed-signers (RO), and `SSH_AUTH_SOCK` so sessions and signing carry over without copying secrets in.
- Per-folder bash history in a named volume.

## Requirements

- Docker (daemon reachable from your user, no sudo).
- `git`, `curl`.
- Linux x86_64 or aarch64.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/filidorwiese/kekkai/main/install.sh | bash
```

Drops a `kekkai` binary in `~/.local/bin/` (no sudo, no shell-rc edits). The installer prints a PATH hint if that directory isn't on your `PATH`.

Pin a specific version: `KEKKAI_VERSION=v0.1.0 curl -fsSL …`.

## Use

From any project folder:

```sh
kekkai up          # build (first time only) + start sandbox, exec claude
kekkai down        # stop and remove the sandbox for this folder
kekkai shell       # open zsh in the running sandbox
kekkai ps          # list running kekkai containers
kekkai prune       # remove orphans (containers, images); --volumes for history vols
kekkai doctor      # diagnose host setup
kekkai version
```

`kekkai up -- <claude args>` appends to the configured Claude args.

## Configuration

Three layers, merged in order:

1. Built-in defaults.
2. `~/.kekkai.yml` — user-wide overrides.
3. `./.kekkai.yml` — project overrides.

Arrays append (extra `apt_packages`, extra `allowed_domains`). Scalars override. `claude.args` replaces. Unknown keys fail with a line number.

### Example `./.kekkai.yml`

```yaml
image:
  apt_packages: [htop, jq]

firewall:
  allowed_domains:
    - registry.terraform.io
    - releases.hashicorp.com

mounts:
  - { source: ~/.aws, target: /home/kekkai/.aws, readonly: true, optional: true }

# Mount the host docker socket so Claude can run `docker` inside the sandbox.
# Off by default — enabling effectively grants host-root via the daemon.
docker_access: true
```
