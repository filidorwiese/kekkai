# Data Model: Update Notice on Up

No persistent data. One transient decision per `up` run:

## Notice decision (transient)

| Input state | Condition | Output |
|-------------|-----------|--------|
| Dev build | `version == "dev"` or no `v` prefix | "" (no network call) |
| Newer available | compareVersions(installed, latest) == -1 | notice line (contracts/update-notice-cli.md) |
| Current | comparison == 0 | "" |
| Ahead | comparison == +1 | "" |
| Inconclusive | any `latestTag` error (offline, timeout, 403/429, malformed) | "" |
| Not ready at print point | goroutine still running when handoff begins | nothing printed this run |

## Inputs

- **Installed version**: `main.version` (ldflags), threaded through `UpOptions.Version` (already exists).
- **Latest release tag**: GitHub `releases/latest` of `repoSlug()` (honors `KEKKAI_REPO`) — identical to self-update.

## Invariants

- At most one notice line per `up` run; only on the success path (before interactive handoff).
- The advertised `<latest>` is byte-identical to what `self-update` would resolve at that moment (same function).
