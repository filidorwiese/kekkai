# Contract: `image.apt_repos` config validation

Scope: `internal/config` — schema acceptance and `Validate()` rejections. All checks run at config load, before any docker work, and report in the same one-pass violation list as every other config error (§4.4).

## Accepted schema

```yaml
image:
  apt_repos:
    - name: dart                                                              # required
      url: https://storage.googleapis.com/download.dartlang.org/linux/debian  # required, https
      suite: stable                                                           # required; trailing "/" = flat repo
      components: main                                                        # optional, default main; forbidden for flat
      key_url: https://dl-ssl.google.com/linux/linux_signing_key.pub          # optional, https
```

Unknown keys inside an entry are rejected by strict parsing (`KnownFields`), like everywhere else.

## Field grammar (allowlist; anything else rejects)

| Field | Grammar |
|---|---|
| `name` | `^[a-z0-9-]{1,64}$` |
| `url` | prefix `https://`, then 1+ chars of `[A-Za-z0-9._~%/+:-]` |
| `key_url` | same as `url` (when present) |
| `suite` non-flat | `^[A-Za-z0-9._+-]+$` |
| `suite` flat | ends with `/`; every `/`-separated segment matches `^[A-Za-z0-9._+-]+$` or the whole suite is `./`; the literal sequence `..` is rejected |
| `components` | `^[a-z0-9-]+$` (single component) |

Structural consequence: `[`, `]`, `=`, whitespace, and all shell metacharacters are unrepresentable in every field ⇒ apt option syntax (`trusted=yes` included) cannot be expressed. There is no unsigned-repo escape hatch by construction (FR-004).

## Rejection conditions and error shape

Every error is prefixed `image.apt_repos[<i>]` (plus `(<name>)` when the name itself is valid) and names the offending field. Conditions:

1. `name` missing, or not matching the grammar.
2. Duplicate `name` across entries (error on the later index, referencing the earlier).
3. `name` is a reserved builtin name: `github-cli`, `githubcli` — message says the name is reserved for the builtin GitHub CLI repository.
4. `url` missing, non-https, or charset violation.
5. `key_url` present and non-https or charset violation.
6. `suite` missing or grammar violation (including `/` appearing in a non-flat suite, i.e. `/` anywhere but as flat segment terminators).
7. Flat entry (`suite` ends `/`) with non-empty `components` — message says components must be omitted for flat repositories.
8. `components` present and not matching the grammar.

Multiple violations across fields/entries all appear in one pass; no fail-fast.

## Non-rejections (explicitly allowed)

- `apt_repos: []` — equivalent to absent; no error, no identity change.
- `key_url` absent — repo relies on system keyrings (backports case).
- Two entries pointing at the same `url` with different names — pointless but harmless.
- `components: ""` on a non-flat entry — behaves as the default `main` (R8).
- `apt_repos` set while `apt_packages` is empty — repos registered, nothing installed from them.
