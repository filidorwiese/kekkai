# Data Model: Custom Apt Repositories (`image.apt_repos`)

## Config entity: `AptRepo`

Lives in `internal/config` as a slice field on `ImageConfig`:

```go
type ImageConfig struct {
    NodeVersion string    `yaml:"node_version"`
    AptPackages []string  `yaml:"apt_packages"`
    AptRepos    []AptRepo `yaml:"apt_repos"`
}

type AptRepo struct {
    Name       string `yaml:"name"`
    URL        string `yaml:"url"`
    Suite      string `yaml:"suite"`
    Components string `yaml:"components"`
    KeyURL     string `yaml:"key_url"`
}
```

| Field | Required | Constraint (validated at load, contracts/config-validation.md) | Notes |
|---|---|---|---|
| `Name` | yes | `^[a-z0-9-]{1,64}$`; unique within list; not `github-cli`/`githubcli` (reserved) | keys keyring + sources filenames and error messages |
| `URL` | yes | https; charset allowlist | repository base URL |
| `Suite` | yes | non-flat: `^[A-Za-z0-9._+-]+$`; flat: ends `/`, slash-terminated segments, no `..` | trailing `/` ⇒ flat repository |
| `Components` | no | `^[a-z0-9-]+$`; must be empty for flat entries | empty on non-flat ⇒ `main` at render time (R8) |
| `KeyURL` | no | https; charset allowlist | present ⇒ key fetched at build, repo pinned via `signed-by`; absent ⇒ system keyrings |

State: entries are immutable value objects; no lifecycle. Zero entries ≡ field absent (identity-neutral, R6).

## Derived render data (internal to `runtime.renderDockerfile`)

Per entry, computed at render — never stored:

| Derived | Value |
|---|---|
| `KeyringPath` | `/etc/apt/keyrings/kekkai-<name>.gpg` (only when `KeyURL` set) |
| `SourcesPath` | `/etc/apt/sources.list.d/kekkai-<name>.list` |
| `Options` | `arch=$(dpkg --print-architecture)` + (` signed-by=<KeyringPath>` when `KeyURL` set) |
| `SuiteLine` | flat: `<suite>`; non-flat: `<suite> <components-or-main>` |

## Identity inputs

- `ImageTag`: unchanged formula; repos flow in via the rendered Dockerfile.
- `ConfigHash`: new input — deterministic serialization, config order, one line per entry: `name|url|suite|components|key_url` (raw config values, components not defaulted). Empty list ⇒ empty string ⇒ hash unchanged vs. today.

## Relationships

- `AptRepo.Name` must not collide with the builtin GitHub CLI repo (reserved names) — the `kekkai-` filename prefix additionally guarantees filesystem separation (R1).
- `image.apt_packages` consumes repos implicitly: registration precedes the final `apt-get update` in the same RUN block (FR-007).
