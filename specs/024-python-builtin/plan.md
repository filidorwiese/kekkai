# Implementation Plan: Builtin Python Runtime

**Branch**: `024-python-builtin` | **Date**: 2026-08-24 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/024-python-builtin/spec.md`

## Summary

Append three packages (`python3`, `python3-venv`, `python3-pip`) to the `builtinAptPackages` constant as a new "scripting runtime" group, and add one line to `embed/Dockerfile.tmpl`'s first root RUN block writing `/etc/pip.conf` with `[global]\nbreak-system-packages = true` — disabling the PEP 668 guard image-wide (FR-002). Verified in a trixie container: 23 packages / 41 MB installed with `--no-install-recommends` (the specs/021 "~350 MB" figure was pip's *recommends* pulling build-essential, which the image flag already suppresses); guard-free `pip install` works as root and as uid-1000 non-root (auto `--user` fallback), `python3 -m venv` produces a working pip, `[global]` section breaks no other pip command. No identity-code changes: packages flow into the rendered Dockerfile (`ImageTag`) and `ConfigHash`; the pip.conf line changes the template text (`ImageTag`) while the simultaneous package-list change covers `ConfigHash`, so the offline fallback can never serve a pre-upgrade image. No firewall change (clarified 2026-08-24): README documents `pypi.org` + `files.pythonhosted.org` as `network.allowed_domains` opt-ins. SPECIFICATION.md §5.1 and §6.3 amended in the same change.

## Technical Context

**Language/Version**: Go (stdlib only), single static binary — unchanged. In-image: Python 3.13.5 (trixie)

**Primary Dependencies**: none new. In-image: 3 Debian packages (+20 transitive), all in trixie main, verified installable on amd64; 12.4 MB download, 41.0 MB installed with `--no-install-recommends` — within the SC-006 120 MB bound

**Storage**: N/A

**Testing**: end-to-end per constitution IV — real `kekkai up`, in-sandbox `python3`/`pip`/`venv` checks via `kekkai exec`; quickstart.md scenarios. Pre-validated in a bare trixie container (research.md)

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (image is linux either way); all packages exist for amd64 and arm64 in trixie main

**Project Type**: CLI tool baking a Docker image from an embedded template

**Performance Goals**: no runtime impact; build time grows by one apt fetch of ~12 MB download

**Constraints**: bake-time inputs only; exactly one rebuild per config on upgrade, then reuse; no new config surface; firewall/sudoers/§5.4 untouched (clarification 2026-08-24: PyPI egress stays per-project opt-in)

**Scale/Scope**: 4 files: `internal/runtime/up.go` (constant), `embed/Dockerfile.tmpl` (one pip.conf line), SPECIFICATION.md (§5.1, §6.3), README.md (pip egress note)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | SPECIFICATION.md §5.1 (builtin list) + §6.3 (pip.conf bullet) amended in the same change as the code (FR-008); README pip-egress note (FR-007) rides along as the user-facing digest | PASS — planned |
| II. Container Is the Security Boundary | §5.4 always-allowed set untouched — PyPI explicitly NOT baked (clarification 2026-08-24); no sudoers change, no daemons; packages fetched at build time on the daemon's host network like every existing builtin. pip.conf relaxes a *local* interpreter guard, not a network or privilege control. Not on the §11 out-of-scope list. | PASS |
| III. Minimal Surface | Zero new config keys or subcommands; one constant extended plus one template line, justified in spec (ad-hoc python is a near-universal agent operation, same bar as specs/021). Non-goals keep compilers/`python3-dev` out. | PASS |
| IV. End-to-End Validation | quickstart.md validates against a real daemon: interpreter/stdlib, guard-free pip install (with pypi in `allowed_domains`), firewall block without it, venv, one-rebuild-then-reuse, firewall probes. | PASS |
| Constraint: image hash from bake-time inputs only | Packages are an existing bake input (Dockerfile render + ConfigHash param). The pip.conf line is template text → enters `ImageTag` by construction; ConfigHash correctness is carried by the simultaneous package-list change (see research.md R4). No identity code changes. | PASS |

**Post-design re-check**: unchanged after Phase 1 — design adds nothing beyond the constant, one template line, and docs. PASS.

## Project Structure

### Documentation (this feature)

```text
specs/024-python-builtin/
├── plan.md                    # This file
├── research.md                # Phase 0 output
├── data-model.md              # Phase 1 output
├── quickstart.md              # Phase 1 output
├── contracts/
│   └── python-runtime.md      # Phase 1 output — package set, pip.conf contract, egress posture
└── tasks.md                   # Phase 2 output (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/runtime/up.go     # builtinAptPackages: new "scripting runtime" group (3 packages)
embed/Dockerfile.tmpl      # first root RUN block: + printf '[global]\nbreak-system-packages = true\n' > /etc/pip.conf
SPECIFICATION.md           # §5.1 builtin list, §6.3 pip.conf bullet
README.md                  # network.allowed_domains example/note: pypi.org + files.pythonhosted.org for pip
```

**Structure Decision**: existing single-binary layout; the feature is a constant extension plus one template line, flowing through the established render → hash pipeline. No new packages, files, or config surface.

## Complexity Tracking

No constitution violations. Table not needed.
