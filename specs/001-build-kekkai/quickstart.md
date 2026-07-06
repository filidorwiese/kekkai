# Quickstart: validating kekkai end-to-end

**Feature**: 001-build-kekkai. Validation runs against a real docker daemon (constitution IV);
there is no mock layer. Contracts: [cli.md](contracts/cli.md), [config.md](contracts/config.md),
[sandbox.md](contracts/sandbox.md).

## Prerequisites

- Linux (amd64 or arm64), Docker daemon running, git, curl, Go 1.24+.
- A throwaway test project directory (git repo for the git-mode checks).

## Build

```sh
go build -ldflags "-X main.version=v0.0.0-dev" -o /tmp/kekkai ./cmd/kekkai
go vet ./...
```

## Scenario 1 — init → up happy path (US1)

```sh
cd ~/kekkai-test && git init -q .
/tmp/kekkai init            # writes .kekkai.yaml; run again → must error
/tmp/kekkai up
```

Expect: image builds (first run), firewall init prints probe results, Claude starts in
`/workspace`. Then exit Claude → `docker ps -a | grep kekkai` shows nothing (container removed).
Re-run `up`: no rebuild (warm start < 15 s to claude exec, SC-001).

## Scenario 2 — validation fails early (US1)

```sh
rm .kekkai.yaml && /tmp/kekkai up          # exact error: no .kekkai.yaml found, run 'kekkai init'
printf 'image:\n  base_image: ubuntu:24.04\nbogus: 1\nnetwork:\n  allow_all: true\n  allow_github: true\n' > .kekkai.yaml
/tmp/kekkai up
```

Expect: one report listing all three violations (non-node base image, unknown key `bogus`,
allow_all+allow_github); `docker images`/`docker ps` untouched.

## Scenario 3 — firewall deny-by-default + probes (US2)

With default config, inside the session (or via `kekkai shell`):

```sh
curl -sS --max-time 5 https://api.anthropic.com   # reaches (HTTP status from server)
curl -sS --max-time 5 https://example.com          # must fail (rejected)
curl -sS --max-time 5 https://registry.npmjs.org   # must fail
```

Add `network: {allow_github: true}` → recreate → `curl https://api.github.com/zen` succeeds.
Add `allowed_domains: [registry.npmjs.org]` → npm registry reachable, example.com still blocked.
Set `allow_all: true` (alone) → warning printed, everything reachable, probes skipped.

## Scenario 4 — disk, secrets, env, limits (US3)

```sh
echo "SECRET=1" > .env.production && mkdir -p deploy/certs && touch deploy/certs/key.pem
# config: secrets.hide: [.env.production, deploy/certs]; limits: {cpus: 1, memory: 1g}
# env: {NODE_ENV: development}; disk.mounts: ~/.aws readonly+optional
```

Inside sandbox: `cat /workspace/.env.production` → empty; `ls /workspace/deploy/certs` → empty;
`echo $NODE_ENV` → development. Host after exit: both secret paths intact, no new artifacts.
`docker inspect` shows cpu/memory limits. Hide a nonexistent path → warn+skip, nothing created.

## Scenario 5 — git modes (US3)

- Default (git omitted): inside — `git log` works, `git commit --allow-empty -m x` fails,
  `git fetch` fails.
- `git: {enabled: true}`: commit succeeds, author matches host `~/.gitconfig`.
- `git: {enabled: true, ssh_agent: true}` with agent running: `ssh-add -l` inside lists host
  keys. Without `SSH_AUTH_SOCK` on host: `up` hard-errors.
- `git: {ssh_agent: true}` without enabled: validation error.

## Scenario 6 — lifecycle across projects (US4)

Two test projects, `up` in both (second terminal):

```sh
/tmp/kekkai ps        # lists both with cwd + image hash
/tmp/kekkai shell     # zsh inside the sandbox for $PWD only
/tmp/kekkai up        # refused (already running); --force recreates
/tmp/kekkai down      # removes only $PWD's container
/tmp/kekkai prune --volumes   # after downs: prompts, removes unused kekkai images + history vols
```

## Scenario 7 — version resolution fallback (US1 edge)

With a warm image and `claude.version: latest`, block registry.npmjs.org on the host (or run
offline): `up` warns and reuses the newest existing image. With version pinned (e.g. `2.0.14`):
no registry call, hash stable.

## Scenario 8 — distribution (US5)

Push tag `v0.0.1-rc1` to a fork → workflow produces linux amd64+arm64 tarballs + `SHA256SUMS` +
release. On a clean machine: `curl -fsSL .../install.sh | sh` → `~/.local/bin/kekkai version`
prints the tag.

## Pass condition

All scenarios behave as stated; any deviation is a spec-or-code bug — reconcile SPECIFICATION.md
and code in the same commit (constitution I).
