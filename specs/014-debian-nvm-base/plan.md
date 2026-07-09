# Implementation Plan: Debian Base Image with Build-Time Node Install (nvm)

**Branch**: `014-debian-nvm-base` | **Date**: 2026-07-09 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/014-debian-nvm-base/spec.md`

## Summary

Replace the `node:<version>-<debian>` base image with a kekkai-pinned `debian:trixie` base plus a build-time Node install via a pinned nvm release. The user's `image.node_version` selector (`lts` | `22` | `22.11` | `22.11.0`) drives `nvm install` inside the Dockerfile; resolved node/npm/npx (and claude) binaries are symlinked into `/usr/local/bin` so every exec path works without profile sourcing. The Docker Hub node-tag probe becomes obsolete and is deleted; validation tightens to the four accepted selector forms; startup output gains the resolved Node version via a CMD echo baked into the image.

## Technical Context

**Language/Version**: Go (single static binary, stdlib `flag`; module `kekkai`)

**Primary Dependencies**: `gopkg.in/yaml.v3` (config), docker CLI (shelled out via `internal/docker`); in-image: nvm v0.40.5 (pinned constant), Node (user-selected), `@anthropic-ai/claude-code`

**Storage**: N/A (docker images/labels are the only persisted state)

**Testing**: End-to-end against a real docker daemon per constitution IV (no unit-test suite of consequence); scenarios in [quickstart.md](quickstart.md)

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (host); linux containers (sandbox)

**Project Type**: CLI (single Go binary `./cmd/kekkai`)

**Performance Goals**: No new docker calls on the happy path; `up` with cached image stays sub-second before handoff; one-time image build cost grows by the nvm+Node download (network-bound)

**Constraints**: Image hash derives from bake-time inputs only (§6.1); build downloads run on host network (firewall does not exist at build time); non-root `kekkai` user UID/GID 1000 preserved; no new builtin firewall destinations

**Scale/Scope**: ~5 files touched in-repo (`embed/Dockerfile.tmpl`, `internal/config/config.go`, `internal/config/validate.go`, `internal/runtime/up.go`, `internal/runtime/init.go`) plus `SPECIFICATION.md` and `README.md`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
|---|---|---|
| I. Spec-First | `SPECIFICATION.md` §4.2, §4.3, §6.1, §6.3 updated in the same commit as the code; README stays the digest (FR-011) | PASS (planned, tracked as tasks) |
| II. Container Is the Security Boundary | No boundary change: build-time downloads happen on the host network **before** the sandbox/firewall exists — documented in code comments (FR-010). §5.4 builtin destination set untouched. No new sudo grants; sudoers block unchanged | PASS |
| III. Minimal Surface | No new subcommand, config key, or Go dependency. Net code shrinks: the Docker Hub tag probe (`baseImageMissing`, ~45 lines) is deleted; two constants (debian base, nvm tag) added | PASS |
| IV. End-to-End Validation | Validation is e2e via quickstart scenarios: build with each selector shape, exec-path probes (`docker exec`, `sh -c`), invalid-selector fail-fast, firewall probes unaffected | PASS |
| Constraint: image hash = bake-time inputs only | Selector string, debian base, nvm tag, apt packages, claude version all enter via the rendered Dockerfile → same hash mechanism. `lts` resolution frozen at build time (selector string hashes, not the resolved version) | PASS |

**Post-Phase-1 re-check**: no violations introduced by the design; Complexity Tracking stays empty.

## Project Structure

### Documentation (this feature)

```text
specs/014-debian-nvm-base/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   ├── node-version.md  # config key contract (accepted forms, errors)
│   └── dockerfile.md    # image contract (layout, exec-path guarantees, startup line)
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
embed/
├── Dockerfile.tmpl        # REWRITE: FROM debian:trixie, kekkai user creation,
│                          #   nvm install (pinned tag), symlinks, version echo in CMD
├── embed.go               # unchanged
└── init-firewall.sh       # unchanged

internal/config/
├── config.go              # ResolvedBaseImage() removed; constants: DebianBaseImage,
│                          #   NvmVersion; NodeInstallArg() selector translation
└── validate.go            # nodeVersionPattern tightened to lts|major[.minor[.patch]];
│                          #   error text names accepted forms

internal/runtime/
├── up.go                  # ensureImage: drop baseImageMissing probe; new template
│                          #   data (DebianImage, NvmVersion, NodeInstallArg);
│                          #   ConfigHash inputs updated
├── identity.go            # ConfigHash doc/param rename only (mechanism unchanged)
└── init.go                # starter config comment: accepted node_version forms

SPECIFICATION.md           # §4.2 schema comment, §4.3 semantics, §6.1 bake inputs,
                           #   §6.3 Dockerfile contract
README.md                  # image section: node_version forms, no node:* mention
```

**Structure Decision**: Existing single-binary layout; no new packages or files besides this feature's docs. All changes are edits to the files listed above.

## Design Decisions (from research)

Summarized here; rationale and alternatives in [research.md](research.md).

1. **nvm pinned at `v0.40.5`** (latest release, 2026-06-04) as a code constant next to the Debian base constant.
2. **Selector translation**: `lts` → `nvm install --lts`; numeric forms pass through verbatim (nvm resolves `22` / `22.11` to the newest matching release). Validation regex `^(lts|[0-9]+(\.[0-9]+){0,2})$`.
3. **`NPM_CONFIG_PREFIX` removed**: nvm refuses to operate with a prefix set; globals land in the user-owned nvm version dir, satisfying FR-008 with no chown dance.
4. **Stable path via `~/.nvm/current` symlink** + root symlinks of `node|npm|npx|claude` into `/usr/local/bin` (FR-005/FR-006); `ENV PATH` prepends `/home/kekkai/.nvm/current/bin` so future runtime `npm install -g` binaries are also reachable.
5. **Resolved-version surfacing (FR-012)**: CMD echoes `kekkai sandbox: node $(node -v), claude <version>` before firewall init — zero extra docker calls, correct even on the §6.2 offline-fallback image.
6. **Nonexistent-version failure (edge case)**: the `nvm install` RUN step wraps failure with an explicit message naming `image.node_version`, so the build error reads as a config problem.
7. **Docker Hub probe deleted**: `baseImageMissing` and its two-request handshake go away; version-existence errors surface at build time (assumption in spec).
8. **`kekkai` user created with UID/GID 1000** via `groupadd`/`useradd` — identical numeric identity to today's renamed `node` user.

## Complexity Tracking

> No constitution violations — table intentionally empty.
