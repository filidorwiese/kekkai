# Research: Sandbox-awareness context

## R1 — Delivery path: dedicated env var + quoted CMD expansion

**Decision**: Carry the prompt in a new builtin env var `KEKKAI_SYSTEM_PROMPT`
and extend the image CMD to
`... && if [ -n "$KEKKAI_SYSTEM_PROMPT" ]; then exec claude $CLAUDE_ARGS --append-system-prompt "$KEKKAI_SYSTEM_PROMPT"; else exec claude $CLAUDE_ARGS; fi`.

**Rationale**: The existing `CLAUDE_ARGS` is expanded *unquoted* in the CMD
(`exec claude $CLAUDE_ARGS`, `up.go`/`Dockerfile.tmpl` §7.2) — by design, it is
a whitespace-separated flag string. A multiline prompt appended there would be
word-split into garbage. A separate var expanded in double quotes arrives
byte-for-byte. The `if` makes the unsupported/skip path launch a byte-identical
command line to today (FR-006). Ordering (`$CLAUDE_ARGS` first, our flag last)
means a user's own `--append-system-prompt` in `claude.args` is preserved as a
distinct occurrence.

**Alternatives considered**: appending the flag+text into `CLAUDE_ARGS`
(word-splitting corrupts it — rejected); shell-escaping the prompt into
`CLAUDE_ARGS` (fragile quoting-inside-quoting across docker -e → sh -c —
rejected); writing the prompt to a container file + `--append-system-prompt-file`
(needs a write location, more moving parts, file flag has its own version
history — rejected); `CLAUDE.md`/`~/.claude` delivery (explicitly forbidden,
FR-004 — rejected).

## R2 — Version gate: threshold v1.0.51

**Decision**: Inject only when the resolved Claude Code version is ≥ `1.0.51`;
below or unknown → skip with one yellow warning. Constant
`appendPromptMinVersion = "1.0.51"` beside the prompt text.

**Rationale**: Verified in the official changelog
(https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md):
v1.0.51 — "`--append-system-prompt` can now be used in interactive mode, not
just --print/-p." Current docs (code.claude.com/docs/en/cli-reference) confirm
the flag works in both modes today and that `--system-prompt`/
`--system-prompt-file` are the *replacement* flags to avoid (FR-001). kekkai
launches interactive claude, so the interactive threshold is the relevant one.

**Unknown-version case**: `ensureImage`'s registry-fallback (reusing the newest
config-matching image when npm is unreachable) leaves the claude version
unknown. Skip + warn there too: passing a flag an old binary rejects would
abort startup — the one outcome FR-006 forbids. Those reused images also
predate any version this feature ships in, making skip almost always correct.

**Comparison**: reuse `internal/selfupdate.compareVersions` (exported as
`CompareVersions`) — exact `[3]int` semver compare already used for the update
notice; no new code, no dependency.

## R3 — `KEKKAI_SANDBOX=1` placement

**Decision**: Builtin env in `buildRunArgs` (§5.3 block), set unconditionally —
independent of the version gate — before user env, therefore user-overridable
like every other builtin.

**Rationale**: The marker is for scripts; it is true regardless of whether the
prompt could be injected. Overridability follows the documented §5.3 convention
("User env below can override") — consistency beats lockdown for an advisory
variable.

## R4 — Prompt constant + FR-008 config summary: implement

**Decision**: `internal/runtime/sandboxprompt.go` with `const sandboxPrompt`
(pinned text verbatim) and `sandboxPromptFor(cfg) string` appending a summary:
allowed domains + CIDRs and `secrets.hide` names, max 8 items per list then
`...and N more`, whole section ≤10 lines, omitted when there is nothing to
list; `allow_all: true` renders `network: unrestricted (allow_all)` instead of
a list.

**Rationale**: FR-008's "if straightforward" test passes: `buildRunArgs`
already holds the parsed `config.Config`; rendering is string concatenation;
the delivery path (R1) is quoting-safe regardless of content. The summary is
what lets Claude answer "is this blocked by the sandbox?" from context instead
of trial and error.

**Alternatives considered**: skipping FR-008 (allowed by spec, but the cost is
~20 lines and the value is the feature's sharpest part — rejected);
`text/template` (overkill for two lists — plain `strings.Builder`).

## R5 — Repeated `--append-system-prompt` occurrences

**Decision**: Do nothing special when the user's `claude.args` also contains
`--append-system-prompt`; both occurrences are passed (user's first, kekkai's
last).

**Rationale**: Official docs show single-use examples and don't document
repeat semantics; empirically recent Claude Code applies repeated append flags
cumulatively, and even if a future version kept only the last occurrence, the
kept one would be kekkai's sandbox context — degraded but safe. Detecting and
merging user text into ours would mean parsing a user flag string kekkai
otherwise treats as opaque — more surface for a corner case (edge case listed
in spec; verified manually in e2e scenario 10 if it arises).
