# Implementation Plan: Custom Apt Repositories (`image.apt_repos`)

**Branch**: `020-apt-repos` | **Date**: 2026-08-20 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/020-apt-repos/spec.md`

## Summary

Add an optional `image.apt_repos` list to `.kekkai.yaml`. Each entry (name, https url, suite, optional components, optional https key_url) renders as a keyring download plus a one-line `deb` entry inside the existing first RUN block of `embed/Dockerfile.tmpl`, registered before the final `apt-get update` so `image.apt_packages` resolves from the new repos in the same build. Flat repos (suite ending `/`) are supported without components. Every field is allowlist-validated at config load (injection surface; `trusted=yes` structurally inexpressible). Image identity follows automatically via the rendered-Dockerfile hash; `ConfigHash` gains the serialized repo list. Build failures with apt signature errors get a hint naming the offending entry's `key_url`. Docs (starter config, README, SPECIFICATION.md) updated in the same change.

## Technical Context

**Language/Version**: Go (stdlib only), single static binary

**Primary Dependencies**: none new. In-image: `curl`, `ca-certificates` (already installed before the repo lines run); apt ≥ 2.4 in debian:trixie auto-detects armored vs binary keys via `signed-by` — no gpg tooling needed

**Storage**: N/A (config file + rendered Dockerfile)

**Testing**: end-to-end per constitution IV — real `kekkai up` against a real docker daemon; quickstart.md scenarios (Dart repo, backports, validation rejections, rebuild trigger, NO_PUBKEY hint)

**Target Platform**: linux/amd64, linux/arm64, darwin/arm64 (image is linux either way); `arch=$(dpkg --print-architecture)` pins each repo to the image arch, same as the builtin GitHub CLI entry

**Project Type**: CLI tool baking a Docker image from an embedded template

**Performance Goals**: no runtime impact; build time grows by one key download + index fetch per repo

**Constraints**: image hash from bake-time inputs only (`apt_repos` is a bake input); zero-repo configs must render a byte-identical Dockerfile (no rebuild churn for existing users); every field is interpolated into the build → allowlist validation only; builtin GitHub CLI repo untouched

**Scale/Scope**: ~5 files: `internal/config/config.go` (+struct), `internal/config/validate.go` (+checks), `internal/runtime/up.go` (+render data, ConfigHash input, build-hint), `internal/docker/cli.go` (BuildImage output capture), `embed/Dockerfile.tmpl` (+range block), `internal/runtime/init.go` (starter), README.md, SPECIFICATION.md

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| I. Spec-First | SPECIFICATION.md (§4.2 schema, §4.4 validation, §6.1 bake inputs, §6.3 Dockerfile contract) amended in the same change as the code (FR-011) | PASS — planned |
| II. Container Is the Security Boundary | Key/package downloads happen at image build time on the host network — same class as existing nvm/npm/GitHub-CLI downloads (§6.3 comment block). Runtime firewall, sudoers, and §5.4 builtin destination set untouched. `trusted=yes` structurally inexpressible (allowlist excludes `[`, `]`, `=`). Not on the §11 out-of-scope list. | PASS |
| III. Minimal Surface | One new config key (a list). Justification: packages outside Debian's repos are simply uninstallable today; structured fields, no raw deb lines, no RUN escape hatch. No new host dependency, no new subcommand. | PASS |
| IV. End-to-End Validation | quickstart.md validates against a real daemon: real third-party repo build, validation rejections, rebuild-on-change, signature-failure hint. Firewall probes untouched. | PASS |
| Constraint: image hash from bake-time inputs only | Repos render into the Dockerfile → `ImageTag` covers them for free. `ConfigHash` (offline-fallback label) extended with the serialized repo list so the fallback never reuses an image baked for different repos. Empty list serializes identically to absent. | PASS |

**Post-design re-check**: unchanged after Phase 1 — design adds no scope beyond the entry list, template block, hash input, and build-hint. PASS.

## Project Structure

### Documentation (this feature)

```text
specs/020-apt-repos/
├── plan.md                       # This file
├── research.md                   # Phase 0 output
├── data-model.md                 # Phase 1 output
├── quickstart.md                 # Phase 1 output
├── contracts/
│   ├── config-validation.md      # Phase 1 output — field grammar + rejection contract
│   └── image-render.md           # Phase 1 output — rendered lines, filenames, hash + hint contract
└── tasks.md                      # Phase 2 output (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/config/config.go     # AptRepo struct on ImageConfig; no new defaults constant
internal/config/validate.go   # apt_repos block in Validate(): grammar, duplicates, reserved names, flat rules
internal/runtime/up.go        # ensureImage: repos → render data + ConfigHash input; buildImage: signature-error hint
internal/runtime/identity.go  # ConfigHash signature gains the serialized repo list
internal/docker/cli.go        # BuildImage tees build output and returns it for hint scanning
embed/Dockerfile.tmpl         # {{range .AptRepos}} keyring fetch + sources line, before final apt-get update
internal/runtime/init.go      # starterConfig: commented apt_repos example (Dart)
README.md                     # image section: apt_repos fields
SPECIFICATION.md              # §4.2, §4.4, §6.1, §6.3 amendments
```

**Structure Decision**: existing single-binary layout; feature threads through the established config → validate → render → hash pipeline. No new packages or files outside docs.

## Complexity Tracking

No constitution violations. Table not needed.
