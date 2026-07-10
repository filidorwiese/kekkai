# Implementation Plan: Pre-Build Node Version Existence Check

**Branch**: `017-prebuild-node-check` | **Date**: 2026-07-10 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/017-prebuild-node-check/spec.md`

## Summary

Restore a fail-fast pre-build check in the slot the feature-014 Docker Hub probe vacated — but against nvm's actual source of truth: `https://nodejs.org/dist/index.json` (same data as the `index.tab` that nvm's `ls-remote` downloads, verified in the pinned v0.40.5 source). Numeric selectors are matched the way nvm matches them (exact for `x.y.z`, prefix for `x`/`x.y`); `lts` skips the check. Tri-state: confirmed-absent aborts pre-build naming `image.node_version`; inconclusive proceeds silently. The in-Dockerfile `|| { echo ERROR ... }` wrap is removed — the installer's native error becomes the only in-build message on the inconclusive path.

## Technical Context

**Language/Version**: Go (single static binary, stdlib `net/http` + `encoding/json`)

**Primary Dependencies**: nodejs.org dist index (read-only GET, ~320KB, verified shape: JSON array of `{"version":"vX.Y.Z",...}`); existing `ensureImage` flow in `internal/runtime/up.go`

**Storage**: N/A (no caching — spec out-of-scope)

**Testing**: E2E per constitution IV: real `kekkai up` runs against the live index (nonexistent version → instant abort; valid → build; unreachable index → silent proceed)

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (host CLI)

**Project Type**: CLI

**Performance Goals**: Cached-image `up`: zero index requests. Build-triggering `up`: one GET, 10s cap (same budget as `resolveLatest`/`latestTag`), no retries

**Constraints**: Check must never block a build (tri-state); config validation stays offline; wrapper removal changes the rendered Dockerfile → one-time rebuild

**Scale/Scope**: 3 files: `internal/runtime/up.go` (probe + call site + template data field removal), `embed/Dockerfile.tmpl` (drop wrap), `SPECIFICATION.md` (§6.1)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
|---|---|---|
| I. Spec-First | `SPECIFICATION.md` §6.1 pre-check paragraph rewritten in the same commit; feature-014 Dockerfile contract superseded via this feature's contract | PASS (planned) |
| II. Container Is the Security Boundary | Host-side pre-build HTTP GET only; no firewall/§5.4 change (build-time network position unchanged) | PASS |
| III. Minimal Surface | No new dependency/config/subcommand. ~35 lines added (probe), ~4 removed (wrap + unused template field). Justified: replaces the worst remaining failure mode; same shape as the accepted 014-era probe but against the true source | PASS |
| IV. End-to-End Validation | Quickstart scenarios run real `up` against the live index, real builds for the fallback path | PASS |
| Constraint: image hash = bake-time inputs | The check is a pre-flight, not a bake input; wrapper removal DOES change the rendered Dockerfile (hash) — expected one-time rebuild, called out in FR-008 | PASS |

**Post-Phase-1 re-check**: no violations; Complexity Tracking empty.

## Project Structure

### Documentation (this feature)

```text
specs/017-prebuild-node-check/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── node-existence-check.md  # check semantics + error text + build-step change
└── tasks.md             # Phase 2 output (/speckit-tasks)
```

### Source Code (repository root)

```text
internal/runtime/
└── up.go                  # nodeVersionMissing(selector) probe (index.json GET,
                           #   10s, tri-state); call in ensureImage inside the
                           #   ImageExists-miss branch, before buildImage;
                           #   NodeVersionRaw removed from template data

embed/
└── Dockerfile.tmpl        # nvm install step: drop `|| { echo ERROR ...; exit 1; }`
                           #   (comment notes the host-side pre-check + native
                           #   nvm error as inconclusive-path fallback)

SPECIFICATION.md           # §6.1: pre-check paragraph (tri-state, index source,
                           #   cached runs request-free, native error fallback)
```

**Structure Decision**: Existing layout; edits only. The probe mirrors the deleted `baseImageMissing` shape (name: `nodeVersionMissing`).

## Design Decisions (from research)

1. **Endpoint `index.json`** (not `index.tab`): same directory, same data as nvm's source; native JSON for Go. Match rule: exact `"v"+sel` for full pins, prefix `"v"+sel+"."` for `x`/`x.y` — mirrors `nvm_ls_remote_index_tab` pattern matching.
2. **Tri-state via boolean "confirmed missing"**: probe returns true only on 200 + parsed + no match; every error path returns false (proceed) — identical semantics to the 014 probe, documented in the function comment.
3. **Placement**: inside `ensureImage`'s `!docker.ImageExists(tag)` branch, before the "building image" print — FR-007 (cached runs request-free) falls out structurally.
4. **Error text**: `image.node_version: %q matches no published Node version — see https://nodejs.org/dist/ for available versions` (contract-pinned).
5. **Wrapper removal**: `nvm install {{.NodeInstallArg}}` left bare (alias/symlink chain unchanged); `NodeVersionRaw` template field deleted (now unused — Minimal Surface).

## Complexity Tracking

> No constitution violations — table intentionally empty.
