# Contract: In-sandbox shell (bash-only)

**Feature**: 015-remove-zsh

## Shell presence

- `bash` is the only general-purpose shell installed by kekkai (`/bin/sh` → dash remains, Debian base).
- `zsh` MUST NOT exist in the image: `command -v zsh` fails; no `.zshrc` is created for the kekkai user.
- A user MAY reinstall zsh via `image.apt_packages` — `kekkai shell` still opens bash.

## Shell surfaces (all MUST agree)

| Surface | Value |
|---|---|
| `kekkai shell` | interactive bash in `/workspace` of the running sandbox |
| kekkai user login shell (`/etc/passwd`) | `/bin/bash` |
| `$SHELL` (container env) | `/bin/bash` |
| CLI help (`kekkai help`, README) | says bash, not zsh |

## History persistence

- `HISTFILE=/commandhistory/.bash_history` (per-project volume, same volume identity as before).
- Incremental append: each command written to the file as it is entered (`PROMPT_COMMAND='history -a'`) — a killed session loses at most the in-flight command.
- Caps: `HISTSIZE=10000` (memory), `HISTFILESIZE=10000` (file).
- Pre-existing volumes: `.bash_history` is created on first use; the old `.zsh_history` is left untouched and unread (accepted one-time visible-history reset).

## Environment

- `POWERLEVEL9K_DISABLE_GITSTATUS` MUST NOT be injected by kekkai. It stays a non-reserved key: a user `env:` entry setting it passes through like any other var.

## Unchanged invariants (regression guard)

- Startup line `kekkai sandbox: node <x.y.z>, claude <ver>`, firewall init gating, sudoers block, CMD structure: byte-identical apart from no zsh being present.
- node/npm/npx/claude exec-path guarantees from feature 014 (contracts/dockerfile.md there) hold on the rebuilt image.
