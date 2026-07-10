# Quickstart Validation: Host UID/GID Match

Prerequisites: Linux host with Docker, host user with uid >= 1000 and gid >= 1000 (ideally uid != gid, e.g. 1000/1001), this repo.

## Build

```sh
go vet ./... && go build -o kekkai ./cmd/kekkai
```

## Scenario 1 — identity match (P1)

```sh
cd /tmp && mkdir -p kekkai-018-test && cd kekkai-018-test
# pseudo-TTY trick from e2e notes; container needs -it
script -qec "/path/to/kekkai up" /dev/null &
sleep 90   # first run builds the image
docker exec $(docker ps -qf label=kekkai.cwd=/tmp/kekkai-018-test) id
```

**Expected**: `uid=<host uid>(kekkai) gid=<host gid>(...)` — numbers equal `id -u` / `id -g` on the host. Startup output must show the firewall verification passing.

```sh
docker exec $(docker ps -qf label=kekkai.cwd=/tmp/kekkai-018-test) touch /workspace/owned-by-me
stat -c '%u:%g' /tmp/kekkai-018-test/owned-by-me
```

**Expected**: `<host uid>:<host gid>` (e.g. `1000:1001`), per [contracts/sandbox-identity.md](contracts/sandbox-identity.md) C1.

## Scenario 2 — rebuild on identity change (P2)

```sh
docker images 'kekkai:*'   # note tag from scenario 1
sudo ./kekkai up           # root → fallback identity
```

**Expected**: a *different* `kekkai:<hash>` image builds (C3); inside it `id` reports 1000/1000 (C2). Clean up the root-built image afterwards.

## Scenario 3 — fallback semantics match pre-018 (C2)

Verify with the fallback identity (1000/1000) the sandbox user is 1000/1000 with group named `kekkai`. Covered by scenario 2's in-container `id`. Note: the rendered Dockerfile is NOT byte-identical to pre-018 (getent guard, numeric chowns), so a one-time rebuild on upgrade is expected for all identities (SC-005).

## Scenario 4 — guarantees hold (C6)

Inside the scenario 1 sandbox:

```sh
docker exec <container> sh -c 'node -v && npm -v && claude --version'
docker exec <container> sh -c 'sudo -l'
```

**Expected**: tooling works without sudo; `sudo -l` lists ONLY `/usr/local/bin/init-firewall.sh`.

## Offline fallback (C4)

Not easily reproducible; verified by inspection: `ConfigHash` includes uid/gid, so `newestImageForConfig` cannot match an image baked for another identity.

## Validation results (2026-07-10, host uid=1000 gid=1001)

- Scenario 1 (C1): PASS — in-sandbox `id` = 1000/1001; host `stat` on sandbox-created file = `1000:1001`.
- Scenario 2/3 (C2, C3): PASS — run under `unshare -r` (uid 0 → fallback): distinct image `kekkai:85d8ff58e0de` vs `kekkai:358b77032256` built automatically; inside `id` = 1000/1000.
- Scenario 4 (C6): PASS — node/npm/claude work unprivileged; `sudo -l` lists only `init-firewall.sh`; firewall verification OK (anthropic reachable, example.com blocked) in both sandboxes.
- C4: PASS — `kekkai.config_hash` labels differ per identity (`139ab1a95ff4` vs `f11414877f3d`).
- C5 collision guard: PASS — scratch build on `debian:trixie` with pre-created gid 1500: build succeeded, `id` = `uid=1500(kekkai) gid=1500(preexisting)`, numeric chowns applied.

## Cleanup

```sh
/path/to/kekkai down; cd /tmp && rm -rf kekkai-018-test
```
