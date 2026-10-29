# Contract: builtin apt package set (specs/029)

## Package set

`builtinAptPackages` gains exactly one entry, `libatomic1`, in a new "node runtime" group after the scripting-runtime group. No entry is removed or reordered.

## Observable guarantees

1. `kekkai up` with `image.node_version` absent, `lts`, `26`, `26.13` or `26.13.0` builds an image whose startup line reads `kekkai sandbox: node v26.x.y, claude <version>` and in which `node -v`, `npm -v` and `claude --version` succeed.
2. The nvm install layer logs the resolved version as current; the strings `Version 'none' does not exist` and `default -> none` never appear in a successful build.
3. Selectors `22` and `24` continue to build and run.
4. Image tag and `kekkai.config_hash` differ from every pre-029 image for the same configuration; the §6.2 offline fallback never selects a pre-029 image.
5. No change to: `embed/Dockerfile.tmpl`, firewall inputs, §5.4 builtin destinations, sudoers, `.kekkai.yaml` schema, `kekkai init` starter.

## SPECIFICATION.md amendment (§5.1)

Insert after the scripting-runtime sentence:

> Node runtime: `libatomic1` — Node ≥ 26 (the LTS line since October 2026) links `libatomic.so.1`, which the Debian base does not ship; without it the nvm-installed node cannot start, `nvm current` resolves to `none`, and the image build fails at the claude install step (specs/029). Baked unconditionally like every other builtin.

And extend the following paragraph's list of path-specific-but-baked packages:

> `jq`/`aggregate` only exercised on the `allow_github` path, `openssh-client` only on the `ssh_agent` path, `libatomic1` only by Node ≥ 26, but all stay baked: the image must be identical regardless of runtime config.
