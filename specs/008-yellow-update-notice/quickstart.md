# Quickstart Validation: Yellow Update Notice

End-to-end per constitution IV. Version is ldflags-injected (feature 005
technique). Rendering table per
[contracts/yellow-notice-cli.md](contracts/yellow-notice-cli.md). Use a test
project whose image is already built so `up` reaches the handoff quickly.

## Scenario 1 — terminal: notice is yellow (US1)

```sh
go build -ldflags "-X main.version=v0.0.1" -o /tmp/kekkai-old ./cmd/kekkai
cd <test project>
script -qec "/tmp/kekkai-old up" /dev/null | cat -A | grep "new version"
```

Expect the notice wrapped in `^[[33m` ... `^[[0m`; text between the codes
matches the 005 contract exactly.

## Scenario 2 — NO_COLOR: plain even in a terminal

```sh
script -qec "NO_COLOR=1 /tmp/kekkai-old up" /dev/null | cat -A | grep "new version"
```

Expect the notice with zero escape sequences.

## Scenario 3 — piped stdout: plain (SC-002)

```sh
/tmp/kekkai-old up | grep -a "new version" | cat -A
```

Expect plain text, no escape codes (non-TTY run ends at the docker TTY
error, after the notice point — the notice may print before it).

## Scenario 4 — silence conditions unchanged (SC-004)

Current build (`-X main.version=<latest>`), ahead build (`v99.0.0`), dev
build (no ldflags), and `KEKKAI_REPO=nonexistent-owner/nonexistent-repo`
with the old build: zero update-related output in all four, as in feature
005 scenarios 2-4.

## Scenario 5 — both advisories yellow in one run

In a dir without any config file, terminal run of the old build: the
missing-config warning (stderr) and the notice (stdout) both render yellow;
`2>/dev/null` drops only the warning, `>/dev/null` (with stderr kept) drops
only the notice.

## Scenario 6 — regression

`go build ./... && go vet ./...` clean; a normal current-version `up` in a
configured project shows no color codes anywhere in piped output.
