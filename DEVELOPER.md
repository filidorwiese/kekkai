# DEVELOPER.md

How to build, run, and test kekkai locally.

## Prerequisites

- Go ≥ 1.22
- Docker
- Linux x86_64 or aarch64

## Build

```sh
go build -ldflags "-X main.version=v0.0.0-dev" -o /tmp/kekkai ./cmd/kekkai
```

Drops the binary at `/tmp/kekkai`. Use that path directly — don't install.

For cross-compile:

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=v0.0.0-dev" \
  -o dist/kekkai-arm64 ./cmd/kekkai
```

## Run

From any project folder you want to sandbox:

```sh
cd ~/some-project
/tmp/kekkai up           # first run: ~3 min image build, then exec claude
/tmp/kekkai down         # remove the sandbox for this folder
/tmp/kekkai shell        # zsh into the running sandbox
/tmp/kekkai ps           # list running kekkai containers
/tmp/kekkai doctor       # diagnose host setup
/tmp/kekkai prune        # remove orphan containers + unused kekkai:* images
```

Forward extra args to claude:

```sh
/tmp/kekkai up -- --model sonnet
```

## Iterating

### Code change → rebuild → run
Go binary changes don't affect the docker image. Just rebuild the binary and run.

### Image-affecting change (Dockerfile.tmpl, init-firewall.sh, image.* defaults)
Image hash = `sha256(rendered Dockerfile + init-firewall.sh)`. Any edit to those files (or to `image.*` values in `embed/defaults.yml`) yields a new hash → new tag → automatic rebuild on next `up`.

To force a rebuild without changing inputs:

```sh
docker image rm kekkai:<hash>   # see `docker images kekkai`
```

### Config change (firewall, mounts, env, docker_access)
Runtime-only, no rebuild. Edit `embed/defaults.yml` (then rebuild the binary), `~/.kekkai.yml`, or `./.kekkai.yml`. `kekkai down && kekkai up` picks it up.

### Test a project-level override
Drop a `./.kekkai.yml` in the target project, e.g.:

```yaml
image:
  apt_packages: [htop]
firewall:
  allowed_domains: [registry.terraform.io]
docker_access: true
```

`/tmp/kekkai up` will rebuild (apt change → image hash change) and apply the runtime overrides.

## Verifying behavior inside the sandbox

After `kekkai up` drops you in claude, `Ctrl+B D` (or exit claude) and `kekkai shell` from another terminal. Quick checks:

```sh
echo $WORKSPACE                          # basename of host $PWD
whoami                                   # → kekkai
ls /home/kekkai/.claude                  # bind from host ~/.claude
curl --max-time 3 https://example.com    # must fail (firewalled)
curl --max-time 3 https://api.github.com/zen   # must succeed (allowlisted)
docker ps                                # only works if docker_access: true
```

## Common pitfalls

- **Stale image not picked up**: confirm the new hash with `docker images kekkai`. If `up` is reusing an old tag, your edit didn't land in a hashed file.
- **`up` refuses with "already exists"**: a previous container is still around. `kekkai down` or `kekkai up --force`.
- **Firewall verification fails on first build**: the script's own check tries `api.github.com/zen`; the host must be able to reach GitHub during image build. Not a kekkai bug.
- **`docker_access: true` errors with "requires /var/run/docker.sock"**: the host socket is at a non-default path (rootless docker?). Not supported yet.
- **`KnownFields` strict YAML error in `~/.kekkai.yml`**: unknown key. Line number is in the error; fix the typo.

## Release dry-run

Before tagging, build both targets locally and confirm size + smoke:

```sh
for arch in amd64 arm64; do
  GOOS=linux GOARCH=$arch CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=v0.0.0-dev" \
    -o dist/kekkai-$arch ./cmd/kekkai
done
ls -lh dist/
file dist/kekkai-*
```

The real release happens by pushing a `v*` tag — `.github/workflows/release.yml` does the matrix build, tars, SHA256SUMS, and creates the GH release.

## Bumping pinned versions

Pinned in `embed/defaults.yml` under `image.*`:

- `git_delta_version`
- `zsh_in_docker_version`
- `tflint_version`
- `docker_cli_version`
- `claude_code_version` (defaults to `latest` — leave alone unless pinning)

Bump = edit + rebuild binary + re-release. Verify upstream release tag exists before changing.

## Layout reminder

```
cmd/kekkai/main.go        subcommand dispatch
internal/config/          load + merge + expand + validate
internal/docker/          exec.Cmd wrappers, label/image/volume lookup
internal/image/           render Dockerfile, hash, build
internal/firewall/        render runtime firewall.conf
internal/runtime/         up / down / shell / ps / prune / doctor / identity
embed/                    Dockerfile.tmpl, init-firewall.sh, defaults.yml
install.sh                curl|bash entry
.github/workflows/        release pipeline
```
