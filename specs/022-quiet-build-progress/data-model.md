# Data Model: Quiet Interactive Build Progress Without Custom Apt Repos

No entities, no config schema change, no stored state. The only datum is a derived mode:

| Derived value | Source | Consumer |
|---|---|---|
| `capture` (bool) | `len(cfg.Image.AptRepos) > 0` at the `buildImage` call site | `docker.BuildImage` writer selection |

Mode semantics (contract in [contracts/build-output.md](contracts/build-output.md)):

- `capture=false` → docker inherits the process stdio; returned build output is `""`; the signature-hint scan receives an empty repo list anyway and stays inert.
- `capture=true` → stdout/stderr teed to a buffer while streaming; buffer returned for the specs/020 hint scan.

`apt_repos: []` yields `capture=false` — consistent with 020's `[]` ≡ absent semantics. Image identity inputs (rendered Dockerfile, firewall script, ConfigHash parameters) are untouched.
