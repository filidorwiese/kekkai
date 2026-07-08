# Research: Update Notice on Up

## R1: Same source of truth as self-update

**Decision**: New exported `selfupdate.Notice(version string) string` reusing the
package's existing unexported internals: `repoSlug()` (honors `KEKKAI_REPO`),
`latestTag()` (GitHub `releases/latest`, 10s timeout), `compareVersions()`. Returns the
notice line only when installed < latest; empty string for dev builds, equal, ahead,
and every error.

**Rationale**: SC-004 requires the notice to advertise exactly what `self-update` would
install; calling the same functions makes divergence impossible. Zero new exports
beyond `Notice`.

**Alternatives considered**: Duplicating the release lookup in `internal/runtime` —
rejected: drift risk, and §3 keeps release logic in `internal/selfupdate`.

## R2: Concurrency — never await

**Decision**: In `runtime.Up`, immediately after config validation succeeds, start
`go func() { ch <- selfupdate.Notice(version) }()` with `ch` buffered (cap 1). Just
before `docker.Interactive(...)`, do a non-blocking `select` read: message present and
non-empty → print; otherwise skip. No `sync`, no timeout plumbing.

**Rationale**: FR-005's "abandoned rather than awaited" maps directly to a
non-blocking channel read. Image ensure typically takes far longer than one HTTPS
round-trip, so the notice appears in practice; when it doesn't (cold cache, instant
startup), silence is the specified behavior. Buffered channel means the goroutine can
never leak blocked; process exit reaps it.

**Alternatives considered**:
- Bounded wait (e.g. select with 500ms timer) at print point — rejected: adds up to
  500ms to the handoff, violating "no noticeable delay" for zero-build startups.
- Check before docker work synchronously — rejected: serial 10s worst case.

## R3: Start point and print point

**Decision**: Start the check after validation passes (not before), so invalid-config
runs never hit the network. Print immediately before the `docker.Interactive` handoff
— after image ensure and preflight, so the notice can never appear amid an error
report (any earlier `return` path skips printing entirely).

**Rationale**: Spec edge case: notice must not mix into error output. Every error path
in `Up` returns before the print point; the notice appears only on the success path.

**Alternatives considered**: Printing at check completion (from the goroutine) —
rejected: interleaves nondeterministically with buildkit output and error reports.

## R4: Dev-build detection

**Decision**: Same predicate `Run` uses: `version == "dev" || !strings.HasPrefix(version, "v")`
→ return "" before any network activity.

**Rationale**: FR-006; identical semantics to self-update's refusal, silent variant.

## R5: Message text

**Decision**: `A new version of kekkai is available (<installed> -> <latest>), run 'kekkai self-update' to upgrade`
printed to stdout, one line, no prefix.

**Rationale**: Names the newer version (FR-002), literally names `kekkai self-update`
(clarification 2026-07-07), matches the `->` arrow style of the self-update success
message. Stdout because it is informational output of a succeeding command, matching
existing `notice:`/`building image` lines.

**Alternatives considered**: stderr — rejected: stderr in this codebase signals
warnings/errors; this is neither.

## R6: Rate limiting / repeated checks

**Decision**: Check on every `up`, no persistence or throttling. GitHub's rate-limit
responses (403/429) already map to an error in `latestTag` → silent skip.

**Rationale**: Spec assumption (no state files, minimal surface). Worst case for a
heavy user is a silently skipped notice — the specified failure mode.
