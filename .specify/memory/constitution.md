<!--
Sync Impact Report
- Version change: 1.0.0 → 1.1.0 (macOS support, specs/002-macos-support)
- Modified principles:
  II  — clarified: builtin destination set (§5.4) changes require spec amendment
  III — platform targets expanded with darwin/arm64; capability-based runtime support
  IV  — added macOS validation tiering (Docker Desktop maintainer-validated)
- Modified sections: Constraints (targets, host prerequisites)
- Templates: ✅ plan/spec/tasks templates unaffected (generic gates)
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
destinations are opened via user config only, never by relaxing `init-firewall.sh`;
changes to the builtin destination set (§5.4) require a spec amendment.
Items in the spec's out-of-scope list (§11) MUST NOT be added without prior discussion.

### III. Minimal Surface

One static Go binary (`./cmd/kekkai`), stdlib `flag`; targets linux/amd64, linux/arm64,
darwin/arm64. On macOS, runtime support is capability-based (preflight probes, §7.4) —
never a per-runtime code matrix; runtime identity may only decorate error messages. One
config file, no layered or user-global config, all defaults as code constants. Less code =
less debt: every new subcommand, config key, or dependency needs justification against the spec.

### IV. End-to-End Validation

Kekkai is distribution infrastructure; no unit-test suite of consequence. Validation is
end-to-end against a real docker daemon: build the binary, run `kekkai up` in a real
project, verify container, mounts, and firewall probes behave as specified. Firewall
verification probes (§9.6) are part of every startup and MUST never be skipped.
macOS tiering: Docker Desktop on Apple silicon is maintainer-validated per release;
other runtimes are capability-probed and community-validated.

## Constraints

- Go, static binary, targets linux/amd64, linux/arm64, darwin/arm64 (Apple silicon;
  Intel Macs and Windows out of scope).
- Host prerequisites: Docker-compatible runtime, git, curl. No other host dependencies.
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

**Version**: 1.1.0 | **Ratified**: 2026-07-06 | **Last Amended**: 2026-07-07
