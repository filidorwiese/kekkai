# Contract: Sandbox runtime (image, container, firewall)

**Source of truth**: SPECIFICATION.md §5–§9.

## Image

- Tag: `kekkai:<sha256(rendered Dockerfile + embed/init-firewall.sh)[:12]>`; built only when
  `docker image inspect` misses. Bake inputs: base image, builtin+user apt packages, resolved
  claude version — nothing else (§6.1).
- `latest` resolved via npm registry pre-render; registry failure → newest existing `kekkai:*`
  image with matching `kekkai.config_hash` label + warning, none → hard error (§6.2). The label
  (version-independent bake-input hash: base image + apt packages + firewall script) is baked
  into every image; it never keys builds (§6.1).
- Dockerfile guarantees (§6.3): base `node:*`; `node` user renamed `kekkai` (UID kept), home
  `/home/kekkai`; npm global prefix `/usr/local/share/npm-global` with claude installed; zsh
  history at `/commandhistory/.zsh_history`; `init-firewall.sh` at `/usr/local/bin/`; sudoers
  contains exactly one grant: `kekkai ALL=(root) NOPASSWD: /usr/local/bin/init-firewall.sh`,
  plus a command-scoped `env_keep` Defaults line whitelisting exactly the four §9 firewall vars
  (sudo env_reset would strip them; SETENV rejected); no docker CLI in image.

## Container

- Name `kekkai-<sanitized-basename(PWD)>-<sha256(PWD)[:8]>`; authoritative key = label
  `kekkai.cwd=$PWD`; also `kekkai.image_hash`, `kekkai.version` (§7.1).
- `docker run --rm -it --cap-add NET_ADMIN --cap-add NET_RAW`, workdir `/workspace`,
  CMD `sudo /usr/local/bin/init-firewall.sh && exec claude $CLAUDE_ARGS` (§7.2).
- Mounts: `$PWD→/workspace` rw; `~/.claude→/home/kekkai/.claude` rw (always);
  history volume→`/commandhistory`; git mounts per §5.2 (enabled: `~/.gitconfig` ro;
  disabled: `$PWD/.git` ro bind if repo — no `SYS_ADMIN`, so unremountable;
  ssh_agent: `$SSH_AUTH_SOCK→/ssh-agent` + `SSH_AUTH_SOCK=/ssh-agent`, allowed_signers ro
  optional); then `disk.mounts`; then secrets shadows.
- Secrets (§8): stat-gated on host pre-run — file → `/dev/null:<path>:ro`, dir → tmpfs,
  missing → warn+skip; docker must never create host artifacts for listed paths.
- Env order (§5.3, §7.3): builtin (`CLAUDE_CONFIG_DIR`, `NODE_OPTIONS`,
  `POWERLEVEL9K_DISABLE_GITSTATUS`, `WORKSPACE`) → user env → firewall env (authoritative) →
  `CLAUDE_ARGS`.
- `limits.cpus`/`limits.memory` → `--cpus`/`--memory`.

## Firewall (`init-firewall.sh`, root via the single sudoers grant)

Inputs: env only — `ALLOW_ALL`, `ALLOW_GITHUB`, `ALLOWED_DOMAINS`, `ALLOWED_CIDRS` (§9).

- `ALLOW_ALL=1`: no restrictions, skip verification, print prominent "egress firewall disabled"
  warning.
- Otherwise: flush (preserving Docker embedded-DNS NAT); allow loopback, udp/53,
  established/related — no blanket port allowances (no global tcp/22); always allow the docker
  bridge subnet (from the container's own route); build `allowed-domains` ipset = builtin hosts
  (`api.anthropic.com` via dig, fatal on failure; `statsig.anthropic.com` warn+skip — it may be
  absent from DNS) + `ALLOWED_DOMAINS` (dig once,
  warn+skip on failure) + `ALLOWED_CIDRS` + GitHub meta CIDRs when `ALLOW_GITHUB=1`
  (jq-validated, aggregated; fetch failure fatal, pre-lockdown); default policy DROP in/out/fwd;
  ipset egress ACCEPT; reject rest with icmp-admin-prohibited.
- **Verification (never disabled)**: `https://example.com` must FAIL; `https://api.anthropic.com`
  must SUCCEED; with `ALLOW_GITHUB=1`, `https://api.github.com/zen` must SUCCEED. Any violation
  → abort before claude starts.
- New destinations only via user config, never by relaxing the script.
