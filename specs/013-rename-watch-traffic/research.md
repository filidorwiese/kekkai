# Research: Rename `kekkai watch` to `kekkai traffic`

## R1 — Name choice: `traffic`

**Decision**: The command is `kekkai traffic`, per the user's clarification
(spec Clarifications, 2026-07-09), superseding both the original request
(`egress-log`) and feature 010's R7 decision (`watch`).

**Rationale**: User decision. `traffic` is self-describing for the stream it
prints; 010's R7 had rejected it on verb-style grounds (`up`, `shell`, `exec`)
— that stylistic argument is now explicitly outweighed by the user's
preference. Recorded so the CLI style precedent is knowingly, not
accidentally, broken.

**Alternatives considered**: `egress-log` (original description — superseded
in clarification); keeping `watch` (rejected by the user).

## R2 — Firewall script comments: rename despite the rebuild

**Decision**: Update the 4 comment lines in `embed/init-firewall.sh` that say
`kekkai watch`, accepting the one-time image rebuild this triggers (the script
is a §6.1 bake-time input). Amend the spec assumption that expected no
rebuild.

**Rationale**: The script is the security-critical artifact reviewers read
most carefully; comments referencing a command that no longer exists would
mislead exactly where accuracy matters most (constitution II review culture).
The rebuild is cheap (the script is COPY'd in a late layer; apt/npm layers
stay cached — project memory) and the feature is pre-release. The diff MUST
be comment-only: any non-comment change to the script fails review (FR-005).

**Alternatives considered**: Leaving the comments (avoids rebuild, honors the
original spec assumption — rejected: permanently stale references in the most
scrutinized file, for a one-time saving); renaming comments in a later
unrelated firewall change (rejected: invisible scope creep for that change).

## R3 — Contract location: new 013 contract, supersession note in 010's

**Decision**: `specs/013-rename-watch-traffic/contracts/traffic-cli.md` is the
living CLI contract (010's contract with exactly three string substitutions);
`specs/010-inspect-egress/contracts/watch-cli.md` gets a one-line header note
pointing at it. SPECIFICATION.md §3 points at the 013 contract.

**Rationale**: Historical artifacts stay intact as records (spec edge case),
but exact strings must live in one authoritative place; a full rewritten copy
under 013 is short and keeps the "exact strings in specs/..." convention
every other feature uses.

**Alternatives considered**: Editing 010's contract in place (rejected:
rewrites history and leaves the 013 feature without its own contract);
duplicating without a supersession note (rejected: two competing "exact
string" sources).

## R4 — Go surface: file and symbol follow the command name

**Decision**: `internal/runtime/watch.go` → `internal/runtime/traffic.go`;
`runtime.Watch()` → `runtime.Traffic()`; comments that name the command
follow. Internal helpers (`watcher`, `event`, parse functions) keep their
names — they describe roles, not the command.

**Rationale**: §3 convention is `internal/runtime/<name>.go` per subcommand;
breaking it for a rename would be the one place the convention silently
fails. `watcher` as a type name still reads correctly (it watches the
stream) and renaming it buys nothing user-visible.

**Alternatives considered**: Keeping `watch.go` with a `Traffic()` func
(rejected: violates the file-per-command convention the spec §3 documents).
