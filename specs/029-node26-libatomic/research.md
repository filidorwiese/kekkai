# Research: Node 26 LTS runs in the sandbox image

All findings verified 2026-10-29 on the Linux host (snap Docker, `debian:trixie` image 826a5616954e dated 2026-08-03) with nvm v0.40.5, the pinned tag in `internal/config/config.go`. arm64 verified via ELF headers of the official tarball, not by execution.

## R1. Root cause

**Finding**: In a plain `debian:trixie` container with only `ca-certificates curl` installed:

```
nvm install --lts          -> "Now using node v26.13.0", default alias created by the installer
nvm current                -> none
node -v                    -> node: error while loading shared libraries: libatomic.so.1:
                              cannot open shared object file: No such file or directory
```

Same sequence with `nvm install 24` → `nvm current` = `v24.22.2`, `node -v` works. Same sequence for v26 after `apt-get install libatomic1` → `nvm current` = `v26.13.0`, `node -v` = v26.13.0, `npm -v` = 11.20.0.

nvm's `nvm_ls_current` (nvm.sh line ~1349) resolves the current version by running `node --version`; an empty result prints `none`. The Dockerfile line `nvm alias default "$(nvm current)" && ln -s "$NVM_DIR/versions/node/$(nvm current)" "$NVM_DIR/current"` therefore aliases `none` (warning only, exit 0) and links `current` → `versions/node/none`. `/usr/local/bin/npm` and the PATH entry both point through `current`, so the next layer sees `npm: command not found`, which the install step's `||` chain reports as a missing claude binary.

**ELF check** (`readelf -d` on the official tarballs):

| Binary | NEEDED includes `libatomic.so.1` |
|--------|-----------------------------------|
| node v26.13.0 linux-x64 | yes |
| node v26.13.0 linux-arm64 | yes |
| node v24.22.2 linux-x64 | no |

So the defect hits every supported container architecture on Node ≥ 26 and no earlier line.

**Decision**: install `libatomic1`. Nothing else is broken.

## R2. Where to add the package

**Options**:

1. `builtinAptPackages` in `internal/runtime/up.go` — baked into every image.
2. Conditional on `image.node_version` resolving to ≥ 26.
3. Document as a user `apt_packages` entry.
4. Switch base image to one that ships it (e.g. `node:*` official images, or `debian:trixie-slim` + extras).

**Decision**: option 1. §5.1 rule: "the image must be identical regardless of runtime config"; `jq`/`aggregate`/`openssh-client` are the precedent for baking a package only one path needs. Package is 45 KB installed, depends on `libc6` and `gcc-14-base`, both already present. Option 2 is code for nothing and leaks node-version logic into the apt layer. Option 3 breaks the no-config default path, which is the primary user story. Option 4 is a platform-constant change with a far larger blast radius (§6.1).

## R3. Hash consequences

**Finding**: `ensureImage` builds `aptPackages = builtin + user`, passes it to `renderDockerfile` (→ `ImageTag` over the rendered text) and to `ConfigHash` (→ `kekkai.config_hash` label). Adding an entry changes both.

- Every user rebuilds once on next `kekkai up`. Expected, and the spec's FR-005 requires it.
- Offline fallback (`newestImageForConfig`) matches on `config_hash`; pre-fix images have a different hash and are never reused. Satisfies the edge case "fallback must not pick a broken image" with no extra code.
- Failed builds tag no image, so hosts that already hit the error have nothing stale to clean up; `kekkai prune` removes the unused older tags as usual.

**Decision**: no migration logic.

## R4. List position and comment

**Decision**: append a new group after the scripting-runtime group:

```go
// node runtime (specs/029): Node >= 26 links libatomic.so.1, absent from
// the Debian base; without it the nvm-installed node cannot start.
"libatomic1",
```

Position affects the hash string only via join order; irrelevant since the hash changes anyway. Grouping by purpose matches the existing style.

## R5. Dockerfile ordering

**Finding**: the first `RUN` installs all apt packages; nvm install runs several layers later as the `kekkai` user. The library is present in the filesystem before `nvm install` executes `node --version`. No template change, no new layer.

**Decision**: template untouched.

## R6. Duplicate user entry

**Finding**: `apt-get install a a` is a no-op for the duplicate. A user who already worked around the bug with `apt_packages: [libatomic1]` keeps building; the duplicate only changes their hash string once.

**Decision**: no dedup logic.
