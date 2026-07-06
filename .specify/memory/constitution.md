<!--
Sync Impact Report
- Version change: none → 1.0.0 (initial ratification)
- Modified principles: n/a (initial adoption)
- Added sections: Core Principles (4), Constraints, Governance
- Removed sections: template placeholder for principle 5 (bare-minimum scope)
- Templates: ✅ .specify/templates/plan-template.md (generic Constitution Check gate, compatible)
             ✅ .specify/templates/spec-template.md (no constitution references, compatible)
             ✅ .specify/templates/tasks-template.md (no constitution references, compatible)
- Follow-up TODOs: none
-->

# Kekkai Constitution

## Core Principles

### I. Spec-First

`SPECIFICATION.md` is the single source of truth for kekkai's design. Design changes MUST
land in the spec first, in the same commit as the code. README stays the user-facing
digest of the spec, never a second source of design.

### II. Container Is the Security Boundary

Docker is the boundary; Claude runs fully autonomous inside it. Anything that punches
through the boundary is rejected, not deferred: no docker socket in the sandbox, no sudo
grants beyond the firewall script, firewall verification never disabled. New network
destinations are opened via user config only, never by relaxing `init-firewall.sh`.
Items in the spec's out-of-scope list (§11) MUST NOT be added without prior discussion.

### III. Minimal Surface

One static Go binary (`./cmd/kekkai`), stdlib `flag`, Linux amd64/arm64 only. One config
file, no layered or user-global config, all defaults as code constants. Less code = less
debt: every new subcommand, config key, or dependency needs justification against the spec.

### IV. End-to-End Validation

Kekkai is distribution infrastructure; no unit-test suite of consequence. Validation is
end-to-end against a real docker daemon: build the binary, run `kekkai up` in a real
project, verify container, mounts, and firewall probes behave as specified. Firewall
verification probes (§9.6) are part of every startup and MUST never be skipped.

## Constraints

- Go, static binary, targets linux/amd64 + linux/arm64.
- Host prerequisites: Docker, git, curl. No other host dependencies.
- Config: strict-parsed `./.kekkai.yaml`, full validation before any docker work,
  all violations reported in one pass.
- Image hash derives from bake-time inputs only (§6.1); runtime config MUST never
  trigger a rebuild.

## Governance

This constitution supersedes ad-hoc practice. Amendments: update this file and
`SPECIFICATION.md` together, bump version per semver (MAJOR: principle removal or
redefinition; MINOR: new principle or section; PATCH: clarification). PRs and reviews
MUST verify compliance, in particular Principle II (boundary integrity) and the spec's
out-of-scope list.

**Version**: 1.0.0 | **Ratified**: 2026-07-06 | **Last Amended**: 2026-07-06
