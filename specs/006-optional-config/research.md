# Research: Optional Configuration File

## R1: Where the missing-file decision lives

**Decision**: `config.Discover` keeps returning `ErrNoConfig`; `config.Load`
keeps returning `(nil, []error{ErrNoConfig})` for a missing file. The fallback
happens at the `up` boundary: `runtime.Up` detects `errors.Is(errs[0],
config.ErrNoConfig)`, prints the warning, and continues with a new exported
`config.Defaults()` (a `*Config` with `applyDefaults()` applied).

**Rationale**: `Load` stays a pure "read what's on disk" function with one
meaning per result; only `up` has the UX mandate to warn-and-continue. A future
caller that needs the file to exist still can. `Defaults()` is the single new
export (constitution III).

**Alternatives considered**: `Load` returning `(Defaults(), nil)` on missing
file — rejected: callers lose the ability to distinguish "no file" from
"empty file", and the warning would need an out-of-band signal anyway.

## R2: Empty and comments-only files

**Decision**: In `Load`, treat `io.EOF` from `Decoder.Decode` as an empty
document: skip the error, fall through to `applyDefaults()`. No warning — the
file's presence is the explicit opt-in; emptiness means "defaults, please"
(spec US2).

**Rationale**: `yaml.v3` returns bare `io.EOF` when the stream contains no
document node, which covers zero-byte files, comments-only files (the new
starter template), and whitespace-only files with one `errors.Is` check.
A `null` document (`~`) already decodes into the zero struct without error and
needs no handling.

**Alternatives considered**: Pre-checking file size / stripping comments before
decode — rejected: duplicate YAML lexing; `io.EOF` is the decoder's own
canonical "empty" signal.

## R3: Warning text, stream, and color

**Decision**: Exactly one line on **stderr**:
`warning: no .kekkai.yaml found, using defaults - run 'kekkai init' to customize`
wrapped in yellow (`\033[33m` ... `\033[0m`) only when stderr is a character
device (`os.Stderr.Stat()` mode contains `os.ModeCharDevice`) and `NO_COLOR`
is unset (https://no-color.org convention). Printed immediately after the
missing-file detection, before any other output.

**Rationale**: stderr because it is advisory, not command output, and must
survive stdout piping (spec edge case). The stat-based TTY check is stdlib-only
(constitution III — no `golang.org/x/term`). `NO_COLOR` is one extra condition
and the accepted accessibility convention. First kekkai color use; no
abstraction built for one call site.

**Alternatives considered**: stdout — rejected: pollutes pipeable output and
this codebase's stdout lines are informational success-path output.
`--quiet` flag or state file to suppress repeats — rejected: new surface,
kekkai keeps zero persistent state (spec assumption).

## R4: Fully commented-out starter template

**Decision**: `starterConfig` in `internal/runtime/init.go`: comment out the
currently active lines (`image:`, `node_version: lts`, `claude:`,
`version: latest`, `args: ...`) so every line is a comment or blank. Header
comment gains one line stating the file is optional and every setting shown is
the default. `kekkai init` success message unchanged.

**Rationale**: FR-006; "uncomment to change" becomes the single gesture, and
the starter can never pin a stale default. The generated file exercises the R2
path (`io.EOF`), which makes US2/US3 the same code path — init-then-up is
covered by the empty-document handling.

**Alternatives considered**: Keeping `image:`/`claude:` as active section keys
with commented children — rejected: bare null sections are legal YAML but
imply structure without content and would skip the R2 path for no benefit.

## R5: Documentation touch points

**Decision**:
- SPECIFICATION.md §3: `init` line loses "(errors if one exists)"? No — that
  behavior is unchanged; only the §4.1 **Required** paragraph is rewritten:
  optional, absent/empty/comments-only = defaults, exact warning line, both
  extensions/conflict rule preserved. §4.5 rewritten: fully commented starter,
  copy/paste-safety paragraph reworded (commented example values must equal
  defaults). `up` line in §3 gains the warning mention.
- README.md: line 74 ("The config file is required ...") and line 78 ("Each
  project needs a .kekkai.yaml ...") rewritten to optional-with-defaults;
  `kekkai init` stays the documented customization path.

**Rationale**: FR-008 and the clarification directive; grep for "required",
"needs a", and the old error string ensures no stale reference.

## R6: Fate of the old hard error

**Decision**: `ErrNoConfig` stays (Discover's not-found signal) but its
message/comment no longer describes a user-facing `up` error; the historical
`specs/001-build-kekkai/contracts/cli.md` is NOT edited (frozen artifact of a
shipped feature); SPECIFICATION.md is the living document that changes.

**Rationale**: Past feature specs are records, not living docs (precedent:
features 004/005 left old spec dirs untouched). The constitution binds
SPECIFICATION.md, which is updated.
