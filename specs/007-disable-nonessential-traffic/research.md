# Research: Disable Nonessential Traffic

## R1: What the switch actually disables

**Decision**: Rely on `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` as the
single umbrella switch.

**Rationale**: Per official Claude Code docs
(code.claude.com/docs/en/env-vars, data-usage, network-config), the variable
bundles `DISABLE_TELEMETRY` (Statsig metrics), `DISABLE_ERROR_REPORTING`
(Sentry), feedback command/surveys, and `DISABLE_AUTOUPDATER`. The only
endpoint Claude still requires is `api.anthropic.com` (API calls + WebFetch
safety check); auth flows (claude.ai / platform.claude.com) happen in a
browser on the host, not in the sandbox — credentials arrive via the
`~/.claude` mount.

**Auto-updater caveat considered**: upstream flags bundling
`DISABLE_AUTOUPDATER` into this switch as a security concern
(anthropics/claude-code#53899) — users could silently miss security updates.
Not applicable here: the Claude version is a bake-time image input, and
`claude.version: latest` (the default) re-resolves on every `up`, so updates
arrive via image rebuild. An in-container self-update would be lost on
container recreation anyway; disabling it is correct, not a regression.

**Alternatives considered**: Setting the four `DISABLE_*` vars individually
(upstream's recommendation when the auto-updater matters) — rejected: the
auto-updater is meaningless in kekkai's image model, the umbrella var is one
line instead of four+, and it is the user's literal request.

## R2: Where the env var is injected and how override works

**Decision**: Add `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` to the
builtin env block in `buildRunArgs` (`internal/runtime/up.go`), before user
env.

**Rationale**: §7.3 ordering is builtin → user → firewall. Docker applies
the last occurrence of a repeated `-e` (verified empirically: `docker run -e
K=a -e K=b` yields `b`), so a user `env:` entry overrides the builtin —
FR-002 falls out of the existing precedence model with zero new code. The
firewall vars stay last and authoritative, unchanged.

**Alternatives considered**: Injecting after user env (kekkai-authoritative,
like firewall vars) — rejected: telemetry is the user's choice on their own
data; the firewall (US2) still blocks statsig regardless of the env value,
so an override grants nothing without an explicit `allowed_domains` entry.
Baking `ENV` into the Dockerfile — rejected: runtime config must not live in
the image (§6.1), and a runtime `-e` keeps the image hash stable.

## R3: Removing statsig from the firewall

**Decision**: Delete `add_domain statsig.anthropic.com warn` from
`embed/init-firewall.sh` and update the adjacent §5.4 comment.

**Rationale**: With telemetry off, the allowance is dead weight — pure
exfiltration surface (constitution II favors the most restrictive posture).
The line is warn-tier with no probe attached, so removal cannot affect the
§9.6 verification. Removing an allowance tightens the script, which
constitution II permits (only *relaxing* is forbidden). Users re-enable via
`allowed_domains: [statsig.anthropic.com]` plus the env override — existing
surface, no new mechanism.

**Alternatives considered**: Keeping the allowance "for compatibility" with
older pinned Claude versions — rejected: their telemetry attempts fail like
any blocked destination and Claude functions normally; keeping an unused
hole contradicts the feature's point.

## R4: Rebuild propagation

**Decision**: No migration code. The firewall script is an input to both
the image hash (§6.1) and the `kekkai.config_hash` label, so every existing
image misses on the next `up` and rebuilds once with the new script.

**Rationale**: This is the designed update path for firewall changes
(precedent: any prior firewall edit). The §6.2 offline fallback also keys on
config_hash, so a registry-unreachable `up` correctly refuses to reuse
old-firewall images for the new config.

**Alternatives considered**: None viable — a label-preserving edit would
defeat the security purpose.

## R5: Documentation touch points

**Decision**:
- SPECIFICATION.md §5.3: add the new builtin env var with a one-line purpose.
- SPECIFICATION.md §5.4: drop statsig; the external builtin set becomes
  `api.anthropic.com` + `host.docker.internal` (warn tier).
- SPECIFICATION.md §9.4 (ipset build step): drop the statsig warn+skip clause.
- README.md network-example comment (~line 121): "api.anthropic.com
  (required by Claude Code) is always allowed".
- `internal/runtime/init.go` starter template network comment: same rewrite.
- README known-limitations bullet: already updated by the user ("Other
  telemetry is disabled inside the sandbox") — verify wording still matches
  the final behavior, no statsig mention.

**Rationale**: FR-005/FR-006 and constitution I/II (spec amendment in the
same commit; README is the digest). Historical specs under `specs/00x-*`
stay frozen (precedent: features 004-006).
