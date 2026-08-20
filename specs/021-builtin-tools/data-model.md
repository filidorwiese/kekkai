# Data Model: Builtin Archive, File-Inspection, and Search Tools

No config schema change and no new entities. The single affected structure:

## Builtin package set (`builtinAptPackages`, code constant)

Purpose-grouped list baked into every image; user `image.apt_packages` appends. This feature appends one group:

| Group (comment) | Packages | Origin |
|---|---|---|
| firewall/lifecycle | sudo, iptables, ipset, iproute2, dnsutils, curl, ca-certificates, jq, aggregate | existing |
| nvm dependency / shell | bash | existing |
| subcommands | tcpdump | existing |
| git over ssh | openssh-client | existing (specs/019) |
| convenience | git, gh, less, nano, procps | existing |
| **general tooling (specs/021)** | **unzip, zip, xz-utils, zstd, bzip2, file, ripgrep, fd-find, rsync** | **this feature** |

Properties:

- Code constant, never user-visible config (§5.1). Immutable at runtime.
- Identity input twice over: serialized into the rendered Dockerfile install line (→ `ImageTag`) and into `ConfigHash` (offline-fallback label). No identity code changes — see research.md R4.
- Canonical-name mapping: every package's command name equals the tool name except `fd-find` → binary `fdfind`, bridged by a template symlink `/usr/local/bin/fd` (research.md R2, contracts/image-tools.md).

## Relationships

- User `apt_packages` entries may duplicate a builtin: apt collapses repeated names in one install invocation; no dedup performed (research.md R5).
- No interaction with `image.apt_repos` (specs/020): builtins install from Debian main only.
