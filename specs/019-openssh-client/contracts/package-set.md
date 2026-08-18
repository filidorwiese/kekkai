# Contract: Sandbox Image Package Set

## Guarantee

Every kekkai sandbox image built by this release contains `openssh-client`, unconditionally.

## Observable behavior

| Check | Command (inside sandbox) | Expected |
|-------|--------------------------|----------|
| Binary present | `command -v ssh ssh-keygen ssh-add` | all three resolve, regardless of `git.ssh_agent` |
| Signing works | `git commit` with host `gpg.format=ssh` config and `ssh_agent: true` | commit created, signed |
| Verify works | `git log --show-signature -1` (with `allowed_signers` mounted) | "Good ... signature" |
| Agent reachable | `ssh-add -l` (with `ssh_agent: true`) | lists host agent keys via `/ssh-agent` |
| SSH remote | `git ls-remote git@github.com:...` (with `allow_github: true` + `ssh_agent: true`) | succeeds via agent auth |

## Identity contract

- Rendered Dockerfile gains ` openssh-client` in the single `apt-get install` line ⇒ image hash and `kekkai.config_hash` change exactly once at upgrade.
- Flipping `git.ssh_agent` between runs produces byte-identical rendered Dockerfiles ⇒ no rebuild.
- User `image.apt_packages: [openssh-client]` duplicate ⇒ build still succeeds.

## Spec linkage

- SPECIFICATION.md §5.1 lists `openssh-client` (same commit as code — constitution I).
- §5.2 mount behavior and §4.4/§7.4 agent-socket validation unchanged.
