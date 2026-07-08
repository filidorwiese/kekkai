# Research: Yellow Update Notice

## R1: Where the color is applied

**Decision**: At the print site in `runtime.Up` — wrap the notice message in
yellow just before `fmt.Println`. `selfupdate.Notice` keeps returning plain
text.

**Rationale**: Color is presentation tied to the destination stream; only the
print site knows the stream. `selfupdate` stays UI-free and its output
remains grep-able by any future caller. The silence logic (non-blocking
select, empty-string check) is untouched — color wraps a message that was
already going to print.

**Alternatives considered**: Coloring inside `selfupdate.Notice` — rejected:
the package would need to know about stdout/TTY/NO_COLOR, and the empty-vs-
colored contract gets murky.

## R2: One helper for both advisories

**Decision**: Extract the feature-006 inline coloring from `warnNoConfig`
into an unexported helper in `internal/runtime`:
`yellow(f *os.File, msg string) string` — returns msg wrapped in
`\033[33m`/`\033[0m` when `f.Stat()` mode has `os.ModeCharDevice` and
`NO_COLOR` is unset, else msg unchanged. `warnNoConfig` passes `os.Stderr`,
the notice print passes `os.Stdout`.

**Rationale**: FR-004 demands the identical yellow and gating; one function
makes divergence impossible and nets less code than a second inline copy
(DRY, constitution III). Parameterizing the stream is required because the
two advisories legitimately live on different streams (005/006 contracts).

**Alternatives considered**: Second inline copy — rejected: two call sites is
the point where duplication starts costing; the 006 research explicitly
deferred abstraction until a second caller existed. A package-level
`color.go` util — rejected: one function, one package, no need.

## R3: TTY semantics for stdout

**Decision**: Same stat-based check as feature 006, applied to stdout.
Notably `docker run -it` handoff is irrelevant: the notice prints from the
kekkai process itself before `docker.Interactive`, so kekkai's own stdout
mode decides.

**Rationale**: Consistency (FR-001/FR-004); stdlib-only (constitution III).
When `up`'s stdout is piped (scripts/CI), the notice stays plain — SC-002.

**Alternatives considered**: Checking stderr's TTY for both lines — rejected:
gating a stdout line on another stream's mode produces color codes in piped
stdout, breaking SC-002.

## R4: Spec touch point

**Decision**: SPECIFICATION.md §3, the `up` update-notice bullet: append that
the line renders yellow when stdout is a terminal and `NO_COLOR` is unset,
same advisory convention as the §4.1 missing-config warning. The frozen
feature-005 contract file is not edited; this feature's own contract
documents the rendering table.

**Rationale**: FR-005, constitution I, and the 004-007 frozen-artifact
precedent.
