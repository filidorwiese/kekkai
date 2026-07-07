# Quickstart: Self-Update, e2e validation guide

Constitution Principle IV: validate against real releases on a real host. Scenarios run on Linux; repeat Scenario 1 on macOS/Apple silicon once a darwin release exists.

## Prerequisites

- At least two published releases on the target repo. For destructive/negative tests use a fork: `export KEKKAI_REPO=<you>/kekkai-test`.
- A scratch install: `KEKKAI_VERSION=<older-tag> KEKKAI_INSTALL_DIR=/tmp/ku sh install.sh`

## Scenario 1 - update available (US1 / SC-001, SC-004)

```sh
/tmp/ku/kekkai version                 # prints <older-tag>
/tmp/ku/kekkai self-update             # "downloading ..." then "Updated kekkai <older-tag> -> <latest-tag>"
/tmp/ku/kekkai version                 # prints <latest-tag>
```

Exit 0; old binary gone, new one executable.

## Scenario 2 - already latest (US2 / SC-003)

```sh
stat -c %Y /tmp/ku/kekkai; /tmp/ku/kekkai self-update; stat -c %Y /tmp/ku/kekkai
```

Output: `You're on the latest version (<latest-tag>)`, exit 0, mtime unchanged, no temp files left in /tmp/ku.

## Scenario 3 - dev build refusal (US3 / FR-007)

```sh
go build -o /tmp/kekkai-dev ./cmd/kekkai && /tmp/kekkai-dev self-update; echo $?
```

Exact dev-build message from [contracts/self-update-cli.md](contracts/self-update-cli.md), exit 1, binary untouched.

## Scenario 4 - unwritable path (US3 / FR-008)

```sh
chmod 555 /tmp/ku && /tmp/ku/kekkai self-update; echo $?; chmod 755 /tmp/ku
```

Fails naming the path BEFORE any "downloading" line, exit 1.

## Scenario 5 - network failure (US3 / FR-009)

```sh
KEKKAI_REPO=filidorwiese/does-not-exist /tmp/ku/kekkai self-update; echo $?
```

`could not determine the latest release ...`, exit 1. For full offline: run with DNS blackholed (`docker run --network none` with the binary) and expect the same class of error.

## Scenario 6 - checksum mismatch (US3 / FR-006) [fork only]

On the fork, edit the release's `SHA256SUMS` asset to corrupt the line, then:

```sh
KEKKAI_REPO=<you>/kekkai-test /tmp/ku/kekkai self-update; echo $?
```

`checksum verification FAILED for <tarball>`, exit 1, installed binary still runs, no temp file remains.

## Scenario 7 - interrupt safety (FR-004)

Run Scenario 1 on a slow link (`trickle` or tc) and Ctrl-C mid-download twice. After each interrupt: `kekkai version` still works (old version), no `.kekkai-update-*` files linger after the next successful run.

## Sign-off checklist

- [X] Scenarios 1-5 pass on Linux (2026-07-07; Scenario 6 pending a fork with an editable release)
- [ ] Scenario 1 repeated on macOS/Apple silicon (once a release with darwin artifact exists)
- [X] Output strings match contract verbatim (SC-004)
- [ ] SPECIFICATION.md §3/§10/§11 amendment in the same commit as the code
