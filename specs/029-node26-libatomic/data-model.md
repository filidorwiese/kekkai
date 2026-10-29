# Data Model: Node 26 LTS runs in the sandbox image

No persistent data. The entities are bake-time inputs and their derived identifiers.

## Builtin package set

`builtinAptPackages` (`internal/runtime/up.go`): ordered list of Debian package names, code constant, grouped by purpose. User `image.apt_packages` is appended after it.

| Group | Entries | Change |
|-------|---------|--------|
| firewall/lifecycle | sudo iptables ipset iproute2 dnsutils curl ca-certificates jq aggregate | — |
| shell | bash | — |
| subcommands | tcpdump | — |
| git over ssh | openssh-client | — |
| convenience | git gh less nano procps | — |
| general tooling | unzip zip xz-utils zstd bzip2 file ripgrep fd-find rsync | — |
| scripting runtime | python3 python3-venv python3-pip | — |
| **node runtime** | **libatomic1** | **added** |

## Derived identifiers

```
aptPackages = builtin ++ user
rendered    = Dockerfile.tmpl(debian base, nvm tag, node selector, aptPackages, apt_repos, claude version, uid:gid, mpr url)
image tag   = kekkai:sha256(rendered + init-firewall.sh + kekkai-mpr.py)[:12]       # keys builds
config_hash = sha256(base \n nvm \n selector \n aptPackages \n firewall \n mpr \n uid:gid \n repos)[:12]  # keys offline fallback
```

Both change with this feature. Consequence table:

| Situation before upgrade | After upgrade, first `kekkai up` | Second `kekkai up` |
|--------------------------|----------------------------------|--------------------|
| Cached image, Node 22/24 | tag miss → one rebuild | cached |
| Failed build (Node 26), no image | tag miss → rebuild succeeds | cached |
| Registry offline, old image present | `config_hash` mismatch → hard error (same as any new config) | — |

## Node selector → runtime library requirement

| Selector resolves to | needs libatomic.so.1 | before | after |
|----------------------|----------------------|--------|-------|
| 22.x, 24.x | no | works | works |
| 26.x (`lts` since 2026-10) | yes (x64 and arm64) | broken | works |
