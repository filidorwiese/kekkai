# Contract: `kekkai version` CLI output

**Feature**: 016-version-update-notice

Extends (does not modify) the specs/005 update-notice contract with a second surface.

## Invocation

`kekkai version` — no flags, no arguments (unchanged).

## Output matrix

| Situation | stdout | stderr | exit |
|---|---|---|---|
| Dev build (`dev` / unstamped) | version string | — (no lookup performed) | 0 |
| Current or ahead of latest | version string | — | 0 |
| Strictly newer release exists | version string | `A new version of kekkai is available (<installed> -> <latest>), run 'kekkai self-update' to upgrade` | 0 |
| Lookup error / timeout / offline / rate-limited | version string | — | 0 |

## Guarantees

- stdout is exactly the version string + newline in every row — `$(kekkai version)` is stable.
- The version line is written before the release lookup starts; worst-case total runtime = existing 10s lookup cap; dev builds do no lookup.
- The stderr line, when present, is byte-identical to the `kekkai up` notice text for the same version pair (single message definition).
- Yellow rendering: ANSI yellow wrapping only when stderr is a terminal and `NO_COLOR` is unset; plain text otherwise. Never any other styling.
- `KEKKAI_REPO` env overrides the release source (existing specs/003 testing hook; applies here identically).

## Non-goals

- No update check on any other subcommand; no caching/throttling; no change to `up`'s notice surface or `self-update` behavior.
