# Quickstart: Debian Base Image with Build-Time Node Install (nvm)

**Feature**: 014-debian-nvm-base

End-to-end validation against a real docker daemon (constitution IV). Contracts under test: [contracts/node-version.md](contracts/node-version.md), [contracts/dockerfile.md](contracts/dockerfile.md).

## Prerequisites

- Docker daemon running; network access (builds download nvm + Node + npm packages).
- Build the binary: `go build -o kekkai-test ./cmd/kekkai`
- Empty test project dir; run all scenarios from it.
- `kekkai up` ends in `docker run -it`: non-TTY runs exit with "the input device is not a TTY" **after** all validation/build logic — sufficient for scenarios 1–4's build/validation assertions. For live-sandbox checks (exec probes, startup line), wrap in a pseudo-TTY:
  `script -qec "kekkai-test up" /dev/null </dev/null &` then poll `docker ps` for the `kekkai-<dir>-<hash>` container.
- Cleanup after each live scenario: `docker rm -f <ctr>`, and remove only this test dir's `kekkai-history-<hash>` volume.

## Scenario 1 — P1: pinned major version, all exec paths (SC-001, SC-004)

```yaml
# .kekkai.yaml
image:
  node_version: "22"
```

1. `kekkai up` (pseudo-TTY) → image builds `FROM debian:trixie`, nvm installs Node 22.x.
2. From the host, with `<ctr>` the running container:
   - `docker exec <ctr> node -v` → `v22.…`
   - `docker exec <ctr> sh -c 'node -v'` → same version (non-login, non-interactive)
   - `docker exec <ctr> zsh -ic 'node -v'` → same version (interactive path)
   - `docker exec <ctr> npm -v` and `docker exec <ctr> npx --version` → succeed
3. `docker exec <ctr> npm install -g cowsay` → succeeds without sudo/permission errors.
4. `docker exec <ctr> sh -c 'command -v cowsay && cowsay ok'` → works (PATH covers runtime globals).
5. `docker exec <ctr> sh -c 'find /home/kekkai -user root | wc -l'` → `0` (no root-owned files in home).
6. Create a file in `/workspace` from inside; on the host it is owned by your user (UID 1000 mapping).

**Expected**: all probes report the same 22.x version; zero permission errors.

## Scenario 2 — P2: zero-config default (lts) + startup line (SC-002, SC-007)

1. Remove `.kekkai.yaml` (wait for the feature-012 remnant: `[ ! -e .kekkai.yaml ] || [ -w .kekkai.yaml ]`).
2. `kekkai up` (pseudo-TTY) → build succeeds; startup output contains one line matching:
   `kekkai sandbox: node v<x.y.z>, claude <x.y.z>` — concrete versions, not selectors.
3. `docker exec <ctr> node -v` → a current LTS major.
4. Repeat with explicit `image.node_version: lts` → **no rebuild** (same selector string → same image hash) and identical behavior.
5. Full pin check: `image.node_version: "22.11.0"` → build succeeds, `node -v` = exactly `v22.11.0` (SC-002 third shape).

## Scenario 3 — P3: invalid/legacy config fails fast (SC-003)

For each config below, run `kekkai up` and confirm: exit before any docker activity (`docker images`/`docker ps` unchanged, no build output), error text per [contracts/node-version.md](contracts/node-version.md):

| Config | Must fail with |
|---|---|
| `node_version: lts/*` | accepted forms named (`lts`, `22`, `22.11`, `22.11.0`) |
| `node_version: node` | same |
| `node_version: stable` | same |
| `node_version: current` | same (previously valid, now rejected) |
| `node_version: ""` | "omit the key for the default \"lts\"" |
| `base_image: node:22` under `image:` | migration error naming `image.node_version` |

## Scenario 4 — edge: well-formed but nonexistent version

```yaml
image:
  node_version: "99"
```

`kekkai up` → validation passes, build starts, `nvm install` fails; the build error output ends with a line naming `image.node_version` (not a bare nvm/Dockerfile error).

## Scenario 5 — no new firewall domains (SC-006), firewall intact

1. Config with only user domains:

```yaml
network:
  allowed_domains:
    - example.org
```

2. `kekkai up` (pseudo-TTY) → image builds successfully (build downloads bypass the not-yet-existing firewall; no nodejs.org/npmjs entries needed).
3. Startup firewall verification probes (§9.6) run and pass unchanged; `example.com` still blocked (never add it to allowed_domains — it is the must-be-blocked probe).

## Scenario 6 — upgrade path (SC-005) + mounts (FR-009)

1. In a project last built with the old node:* base and a supported-keys-only config: `kekkai up` → rebuild triggers automatically (new bake inputs → new hash), zero config edits needed.
2. With `git.enabled: true` and a `~/.claude` present: `docker exec <ctr> ls /home/kekkai/.claude /home/kekkai/.gitconfig` → mounts resolve as before.
3. `kekkai shell`, `kekkai exec node -v`, `kekkai traffic` still function against the new image.

## Sign-off checklist

- [ ] SC-001 all three exec paths report 22.x (Scenario 1)
- [ ] SC-002 lts / major / full builds succeed (Scenarios 1, 2)
- [ ] SC-003 every invalid selector + base_image fails pre-docker (Scenario 3)
- [ ] SC-004 npm -g without sudo, host-owned workspace files (Scenario 1)
- [ ] SC-005 zero config changes on upgrade (Scenario 6)
- [ ] SC-006 no new allowed_domains needed (Scenario 5)
- [ ] SC-007 startup line shows concrete node + claude versions (Scenario 2)
