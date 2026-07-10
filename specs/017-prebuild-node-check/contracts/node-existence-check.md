# Contract: Pre-build Node version existence check

**Feature**: 017-prebuild-node-check

Supersedes the feature-014 in-build error-wrap behavior (contracts/dockerfile.md there, "Install failure wraps..." line).

## When the check runs

- Only during `kekkai up` runs that are about to build (image inspect miss), after claude-version resolution, before any docker build activity.
- Never on cached-image runs (zero index requests).
- Never for `image.node_version: lts` (or omitted key → `lts`).

## Source and matching

- Source: `https://nodejs.org/dist/index.json` — the same dataset nvm's remote resolution consults.
- Matching (mirrors nvm):

| Selector | Exists when |
|---|---|
| `22.11.0` | index contains exactly `v22.11.0` |
| `22.11` | index contains any `v22.11.*` |
| `22` | index contains any `v22.*` |

## Outcomes

| Outcome | Behavior |
|---|---|
| Confirmed absent | abort before build, exit non-zero, error: `image.node_version: "<value>" matches no published Node version — see https://nodejs.org/dist/ for available versions` |
| Confirmed present | build proceeds |
| Inconclusive (timeout 10s / transport / non-200 / malformed) | build proceeds; no output about the check |

## Build-step change (US3)

- The rendered Dockerfile's nvm install step carries NO kekkai-added failure wrapper.
- On the inconclusive-path failure, build output shows nvm's native error only (e.g. `Version '99' not found - try \`nvm ls-remote\` to browse available versions.`) — exactly one error message.
- Rendered-Dockerfile change → image hash change → one-time rebuild on next `up` after upgrading.

## Non-goals

- No caching of index responses; no per-architecture artifact validation; no custom mirror support; no check for `claude.version` (own mechanism exists).
