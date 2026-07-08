# Quickstart Validation: Update Notice on Up

End-to-end per constitution IV. Version is ldflags-injected, so each scenario builds a
binary with a chosen version and runs a real `kekkai up`.

## Prerequisites

- Docker daemon, network access, valid test project (e.g. the one from feature 004:
  `kekkai init` in a scratch dir) whose image is already built (so `up` reaches the
  handoff quickly)
- Latest published release tag: `curl -s https://api.github.com/repos/filidorwiese/kekkai/releases/latest | jq -r .tag_name`

## Scenario 1 — older build shows exactly one notice (US1)

```sh
go build -ldflags "-X main.version=v0.0.1" -o /tmp/kekkai-old ./cmd/kekkai
cd <test project> && /tmp/kekkai-old up      # exit the session normally
```

Expect exactly one line matching the contract
(`A new version of kekkai is available (v0.0.1 -> <latest>), run 'kekkai self-update' to upgrade`)
before the interactive handoff. Startup otherwise identical.

## Scenario 2 — current and ahead builds stay silent (US2)

```sh
go build -ldflags "-X main.version=<latest>" -o /tmp/kekkai-cur ./cmd/kekkai
go build -ldflags "-X main.version=v99.0.0" -o /tmp/kekkai-ahead ./cmd/kekkai
```

Run `up` with each: output contains no update-related text (`grep -i "new version"` empty).

## Scenario 3 — dev build silent, no release call (US3/FR-006)

```sh
go build -o /tmp/kekkai-dev ./cmd/kekkai     # version defaults to "dev"
/tmp/kekkai-dev up
```

No update-related output. (Optionally verify no api.github.com traffic via
`strace -f -e trace=network` or a temporary `KEKKAI_REPO` pointing at an invalid host —
still silent either way.)

## Scenario 4 — unreachable release service never blocks or warns (US3)

```sh
KEKKAI_REPO=nonexistent-owner/nonexistent-repo /tmp/kekkai-old up
```

`up` proceeds normally, zero update-related output, no added wait (404 → silent).
For a network-level failure, run scenario 1's binary with DNS to api.github.com broken
(e.g. in a netns) — same expectation.

## Scenario 5 — notice never appears amid errors (edge case)

```sh
printf 'image:\n  node_version: ""\n' > .kekkai.yaml && /tmp/kekkai-old up
```

Validation error report only — no notice line anywhere (check starts after validation).

## Scenario 6 — exit status unaffected (SC-003)

Compare `echo $?` for scenarios 1, 2, 4: identical to a build without the feature
(success path exit code, TTY caveats aside).
