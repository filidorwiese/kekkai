# Data Model: Builtin Python Runtime

**Feature**: 024-python-builtin | **Date**: 2026-08-24

No new runtime entities, config keys, or state. Two existing bake-time entities are extended.

## Builtin package set (`builtinAptPackages`, internal/runtime/up.go)

Code-constant string list, grouped by purpose. Gains a new group:

| Group | Packages | Added by |
|-------|----------|----------|
| firewall/lifecycle | sudo, iptables, ipset, iproute2, dnsutils, curl, ca-certificates, jq, aggregate | 001 |
| shell / nvm dep | bash | 014/015 |
| subcommands | tcpdump | 010 |
| git over ssh | openssh-client | 019 |
| convenience | git, gh, less, nano, procps | 001 |
| general tooling | unzip, zip, xz-utils, zstd, bzip2, file, ripgrep, fd-find, rsync | 021 |
| **scripting runtime** | **python3, python3-venv, python3-pip** | **024 (this feature)** |

**Validation rules**: unchanged — the list renders into the Dockerfile apt line (ImageTag input) and is a ConfigHash parameter; user `apt_packages` appends after it; duplicates are apt no-ops.

## Baked pip configuration (embed/Dockerfile.tmpl)

New fixed file written at bake time, root-owned:

| Attribute | Value |
|-----------|-------|
| Path | `/etc/pip.conf` (pip's global-config path, lowest precedence tier) |
| Content | `[global]` newline `break-system-packages = true` |
| Written by | one `printf` in the existing first root RUN block |
| Identity | template text → ImageTag input by construction; not a ConfigHash input (rides the simultaneous package change, see research.md R4) |
| Overridable | at runtime only, by higher-precedence pip config (`~/.config/pip/pip.conf`, `PIP_CONFIG_FILE`, CLI flags) — deterministic precedence, nothing in kekkai writes those |
| Scope of effect | disables the PEP 668 externally-managed-environment guard for system/user installs; inert inside venvs (no marker there) |

## Relationships / state transitions

None. Both entities are immutable image-definition inputs; the only observable transition is the standing one: changed bake inputs → new ImageTag → one rebuild per project, then cache reuse (spec FR-005).
