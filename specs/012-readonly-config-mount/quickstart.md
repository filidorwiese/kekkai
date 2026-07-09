# Quickstart: Validating Read-Only Config Mount

End-to-end validation against a real docker daemon (Constitution IV). Message/mount specs: [contracts/config-mount.md](contracts/config-mount.md).

## Prerequisites

```sh
go build -o /tmp/kekkai ./cmd/kekkai   # or: task build
mkdir -p /tmp/kekkai-e2e && cd /tmp/kekkai-e2e && git init -q .
```

Note: `kekkai up` needs a TTY. For non-interactive validation wrap it:
`script -qec "/tmp/kekkai up" /dev/null` (see memory: pseudo-TTY trick).

## Scenario 1: typo file → warn and exit (Story 3)

```sh
touch .kekkai.yml
/tmp/kekkai up; echo "exit=$?"
```

**Expected**: message per contract §1 naming `.kekkai.yml` and the rename; `exit=1`; `docker ps -a` shows no new kekkai container; no image build started.

Also verify:
- Both files present (`touch .kekkai.yaml`): same typo error, still exit 1.
- `kekkai init` with `.kekkai.yml` present: typo refusal per contract §2.
- Cleanup: `rm .kekkai.yml`.

## Scenario 2: config present → readable, immutable (Story 1)

```sh
/tmp/kekkai init
script -qec "/tmp/kekkai up" /dev/null &   # or run interactively
```

Inside the sandbox (or via `kekkai exec` / `docker exec`):

```sh
cat /workspace/.kekkai.yaml            # matches host file
echo hack >> /workspace/.kekkai.yaml   # fails: read-only
rm /workspace/.kekkai.yaml             # fails: busy
mv /workspace/.kekkai.yaml /tmp/x      # fails: busy
```

**Expected**: read OK; every mutation fails; after `kekkai down`, host `.kekkai.yaml` byte-identical (compare sha256 before/after).

## Scenario 3: no config → comment-only placeholder (Story 2)

```sh
rm .kekkai.yaml
script -qec "/tmp/kekkai up" /dev/null &
```

Inside the sandbox:

```sh
cat /workspace/.kekkai.yaml    # exactly the placeholder comment line (contract §4)
echo x > /workspace/.kekkai.yaml   # fails
```

**Expected**:
- Startup still prints the unchanged no-config advisory (contract §5).
- In-container file content = placeholder comment, nothing else.
- Host: during the run the workspace shows only the empty mountpoint remnant docker materializes (contract §4); after `kekkai down`, no `.kekkai.yaml` in the workspace and no leftover `config-*` dir under `<UserCacheDir>/kekkai/`.

## Scenario 4: regression sweep

- Normal `up` with valid config: firewall verification probes still pass (§9.6, never skipped).
- Editing `.kekkai.yaml` (e.g. add allowed domain) does NOT trigger an image rebuild (§6.1); rebuild only on firewall/apt/node/claude changes.
- `secrets.hide` entries still shadowed; hidden paths unreadable in container.
- `go vet ./... && go build ./...` clean.

## Docs check

- SPECIFICATION.md §4.1 no longer mentions `.kekkai.{yml,yaml}` dual acceptance; typo rejection documented; §5.2 lists the config ro mount + placeholder.
- README Configure section states config is visible read-only inside the sandbox and only `.kekkai.yaml` is read.
