# Implementation Plan: Verify Claude Binary at Image Build Time

**Branch**: `023-verify-claude-binary` | **Date**: 2026-08-24 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/023-verify-claude-binary/spec.md`

## Summary

The `npm install -g @anthropic-ai/claude-code` RUN step in `embed/Dockerfile.tmpl` becomes install → verify → retry-once → verify → fail-loud. Verification is `claude --version`: since claude-code 2.x the npm package is a thin wrapper whose native binary arrives via a platform optionalDependency that npm drops silently on transient fetch failure; `claude --version` execs that native binary, so it exits 0 only when the install is actually functional (verified: exit 1 + "native binary not installed" on a broken install, exit 0 in 77ms with `--network none` on a complete one). On double failure the step echoes one actionable line (transient cause, rerun `kekkai up`) and exits nonzero — BuildKit prints the failing step's output in every progress mode, so no host-side Go changes are needed. Docker never tags a failed build, so the next `up` rebuilds instead of reusing a broken image. One template file + one SPECIFICATION.md amendment (§6.3, Principle I); Dockerfile text change → new image hash → the expected one-time rebuild; `kekkai.config_hash` inputs untouched → §6.2 offline fallback semantics unchanged.

## Technical Context

**Language/Version**: Go (stdlib only) for kekkai itself — no Go changes; the feature is Dockerfile template text (`embed/Dockerfile.tmpl`) plus spec prose

**Primary Dependencies**: none new; relies on behavior already in the image: `claude` on PATH within the install RUN step (nvm bin dir via `BASH_ENV`-sourced nvm.sh), BuildKit printing a failed step's output

**Storage**: N/A

**Testing**: end-to-end per constitution IV — real builds: happy path (build succeeds, `claude --version` in-image), simulated failure (native package forcibly absent → build fails, message shown, no tag), retry observation, hash-change rebuild, offline-fallback non-regression

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 hosts; verification runs inside the linux build container on all of them — no platform branch

**Project Type**: CLI tool shelling out to the docker CLI; change is in the embedded Dockerfile template

**Performance Goals**: successful-build overhead ≈ one `claude --version` exec (~0.1s, measured) — within spec SC-004's "few seconds"

**Constraints**: verification must be network-free on a complete install (FR-005, verified with `--network none`); no image-hash semantics change beyond Dockerfile content (FR-006); offline-fallback path untouched (FR-007 — it never builds); actionable failure text (FR-003); exactly one retry (FR-004)

**Scale/Scope**: 2 files: `embed/Dockerfile.tmpl` (install step gains verify+retry+fail-loud), `SPECIFICATION.md` (§6.3 claude-install bullet amended, same commit per Principle I)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | §6.3's "claude installed via npm install -g …" bullet is amended in the same commit to document verify + single retry + fail-loud message (research.md R6). | PASS |
| II. Container Is the Security Boundary | Build-time only; firewall, sudoers, destinations, runtime privileges untouched. Verification runs the just-installed binary inside the build container on the daemon's host network — the same trust already extended to `npm install` itself (§6.1: firewall does not exist until container start). | PASS |
| III. Minimal Surface | No config keys, no flags, no Go code; a few lines of shell in one RUN step. Retry count is a fixed 1, not configurable. | PASS |
| IV. End-to-End Validation | quickstart.md: real-daemon scenarios incl. forced-failure build (no tag, message shown), happy-path build, offline `claude --version`, §6.2 fallback non-regression. | PASS |
| Constraint: image hash from bake-time inputs only | Only the rendered Dockerfile text changes (normal recipe change → one rebuild); no runtime input enters the hash; `kekkai.config_hash` formula untouched, so old and new images stay interchangeable for the offline fallback. | PASS |

**Post-design re-check**: unchanged after Phase 1. PASS.

## Project Structure

### Documentation (this feature)

```text
specs/023-verify-claude-binary/
├── plan.md                        # This file
├── research.md                    # Phase 0 output
├── data-model.md                  # Phase 1 output
├── quickstart.md                  # Phase 1 output
├── contracts/
│   └── claude-install-verify.md   # Phase 1 output — install/verify/retry step contract
└── tasks.md                       # Phase 2 output (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
embed/Dockerfile.tmpl    # claude install RUN step: install && verify || (retry && verify) || fail-loud
SPECIFICATION.md         # §6.3 claude-install bullet amended (same commit, Principle I)
```

**Structure Decision**: existing single-binary layout; the change is confined to the embedded Dockerfile template (rendered text is the contract surface, hashed per §6.1). No `internal/` Go changes: failure surfacing rides on BuildKit's failed-step output (research.md R3), deliberately not on the specs/020 capture/hint machinery, which stays scoped to apt signature failures.

## Complexity Tracking

No constitution violations. Table not needed.
