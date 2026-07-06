# Contract: `.kekkai.yaml`

**Source of truth**: SPECIFICATION.md §4; field/validation matrix in
[data-model.md](../data-model.md). This contract fixes observable behavior of config handling.

## Discovery

- Location: workspace root only. Accepted names: `.kekkai.yml`, `.kekkai.yaml`.
- Both present → error. Neither → `up` errors with `no .kekkai.yaml found, run 'kekkai init'`.
- No layering: no user-global file, no env-based overrides; defaults are code constants.

## Parsing

- `yaml.v3`, `KnownFields(true)`: any unknown key at any level → validation error naming the key.
- Legacy keys `image.base`, `image.claude_code_version`, `firewall`, `docker_access`, top-level
  `mounts` → targeted error: schema changed, run `kekkai init`, see README.

## Validation (before any docker work; all violations in one report)

1. `image.base_image` present and matches `node:*`.
2. `claude.version` is `latest` or an exact npm version string.
3. Every mount has `source`; expanded targets unique.
4. `${VAR}` in mount source / env value with `VAR` unset → error, unless the mount is
   `optional: true`.
5. `env` must not set reserved keys: `WORKSPACE`, `ALLOW_ALL`, `ALLOW_GITHUB`,
   `ALLOWED_DOMAINS`, `ALLOWED_CIDRS`, `SSH_AUTH_SOCK`.
6. `git.ssh_agent: true` requires `git.enabled: true`. At `up`, host `$SSH_AUTH_SOCK` unset →
   hard error.
7. `network.allow_all: true` with any other `network` key set → error. Omitted `network` block
   → firewall on, builtins only.
8. Each `allowed_cidrs` entry parses as CIDR; `allowed_domains` entries contain no whitespace.
9. `limits.cpus` > 0; `limits.memory` matches `^[0-9]+(\.[0-9]+)?[bkmg]?$` case-insensitive
   (docker `--memory` grammar, checked pre-docker).

## Starter file (`kekkai init`)

- Active keys: `image.base_image: node:22-bookworm`, `claude.version: latest`,
  `claude.args: --dangerously-skip-permissions` — values identical to code defaults
  (copy/paste safety, §4.5).
- Every optional section present but fully commented, with explanatory comments; commented
  `GH_TOKEN: ${GH_TOKEN}` example adjacent to `allow_github`.
- Behavior-changing examples (e.g. `--model`) only in comments.
