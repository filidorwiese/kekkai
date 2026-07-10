# Contract: Sandbox Identity

## User-observable contract

| # | Given | Then |
|---|-------|------|
| C1 | Linux host, uid >= 1000 and gid >= 1000 | `id` inside sandbox reports exactly host uid/gid; files created in `/workspace` are owned uid:gid on the host |
| C2 | Host uid < 1000 or gid < 1000 (root, system ids, macOS defaults) | Sandbox user is 1000/1000 — semantically identical to pre-018 (one-time rebuild from the template change itself is expected, per SC-005) |
| C3 | Host identity changes for the same project | Next `up` builds/uses a different image automatically; no manual cleanup |
| C4 | Offline fallback (§6.2) engages | Only images whose `kekkai.config_hash` embeds the current identity are eligible |
| C5 | Host gid pre-exists as a group in the image | Build succeeds; sandbox user's numeric gid equals host gid (group name may differ from `kekkai`) |
| C6 | Any rendered identity | Sudoers grant, nvm/Node tooling, `/commandhistory` persistence, ro config mount behave exactly as pre-018 |

## Internal API contract

```go
// internal/runtime — new
// sandboxIdentity returns the uid/gid to bake into the image: the host
// identity when both are in the user range (>= 1000), else the historical
// 1000/1000 fallback (root, system ids, macOS). Pure; no config input.
func sandboxIdentity() (uid, gid int)

// internal/runtime/identity.go — changed signature
func ConfigHash(nodeVersion string, aptPackages []string, firewallScript string, uid, gid int) string
```

`renderDockerfile` template data gains `Uid int`, `Gid int`.

## Template contract (`embed/Dockerfile.tmpl`)

```dockerfile
RUN (getent group {{.Gid}} >/dev/null || groupadd -g {{.Gid}} kekkai) \
 && useradd -m -u {{.Uid}} -g {{.Gid}} -s /bin/bash kekkai
```

All subsequent build-time `chown` use numeric `{{.Uid}}:{{.Gid}}`.

## Out of contract

- Supplementary host groups (not mirrored).
- Matching identities below the 1000 range (fallback applies).
- macOS ownership mapping (owned by the runtime's file-sharing layer).
