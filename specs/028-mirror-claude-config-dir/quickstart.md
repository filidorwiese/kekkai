# Quickstart: validate specs/028 end-to-end

Prereqs: docker daemon, `task build` (binary at `/tmp/kekkai`), a project dir with
or without `.kekkai.yaml`, at least one plugin enabled on the host
(`claude plugin list` non-empty).

## 1. Plugins load (US1)

```sh
cd ~/Projects/some-project
/tmp/kekkai up
```

In the sandbox: type `/`. Expected: host plugin commands listed. Then `/exit`.

```sh
/tmp/kekkai up
# in sandbox shell via a second terminal:
/tmp/kekkai exec claude plugin list
```

Expected: same plugin set as `claude plugin list` on the host.

## 2. Absolute-path hook (US1)

Host: add a hook in `~/.claude/settings.json` whose command is
`~/.claude/hooks/ping.sh` written as an absolute path, script writes a marker to
`/tmp/hook-ran` inside the sandbox. Run `up`, trigger the hook event. Expected:
`/tmp/kekkai exec cat /tmp/hook-ran` succeeds.

## 3. Relocated config dir (US2)

```sh
mkdir -p /tmp/cc && cp -r ~/.claude/settings.json /tmp/cc/
CLAUDE_CONFIG_DIR=/tmp/cc /tmp/kekkai up
/tmp/kekkai exec sh -c 'echo $CLAUDE_CONFIG_DIR; ls /tmp/cc'
```

Expected: `/tmp/cc` printed, host content listed. `docker inspect` of the
container shows a bind `/tmp/cc:/tmp/cc` rw.

## 4. Symlink (US3)

```sh
/tmp/kekkai shell
ls -la ~/.claude ; readlink ~/.claude
```

Expected: listing shows host content; readlink prints the host path.

## 5. Validation refusals

```sh
# disk mount shadowing the config dir
cat > .kekkai.yaml <<Y
disk:
  mounts:
    - source: /tmp
      target: $HOME/.claude/plugins
Y
/tmp/kekkai up
```

Expected: one-pass report containing `would shadow the Claude config dir`, no
docker work. Repeat with `CLAUDE_CONFIG_DIR='/tmp/a:b'`; expected: `contains ':'`
refusal.

## 6. Image parity (SC-005)

```sh
docker images kekkai --format '{{.Tag}}'
CLAUDE_CONFIG_DIR=/tmp/cc /tmp/kekkai up   # exit immediately
docker images kekkai --format '{{.Tag}}'
```

Expected: identical tag list, no rebuild.

## 7. Nested project (edge)

```sh
cd ~ && /tmp/kekkai up
/tmp/kekkai exec sh -c 'touch ~/.claude/.w && rm ~/.claude/.w && echo ok'
```

Expected: `ok` (config dir writable while nested in the project bind).

## 8. macOS

Run steps 1 and 3 on Docker Desktop. With `/tmp/cc` outside shared folders,
expected: preflight `bind` failure naming the Docker Desktop hint.

## Results (2026-09-14, linux/amd64, docker 29.6.1 snap, claude 2.1.270)

| Step | Result |
|------|--------|
| 1 plugins | PASS: `claude plugin list` in sandbox identical to host (3 entries); `-v /home/fili/.claude:/home/fili/.claude` rw; `CLAUDE_CONFIG_DIR=/home/fili/.claude` |
| 2 abs-path hook | NOT RUN: host hooks use `~/...`, settings.json left untouched; hook script confirmed reachable at its host path inside the sandbox |
| 3 relocated | PASS with `CLAUDE_CONFIG_DIR=$HOME/kk028-cc` (bind rw at same path, env set, content listed, writable). `/tmp/cc` shows empty on this host: snap docker has a private `/tmp`, host quirk not feature |
| 4 symlink | PASS: `readlink ~/.claude` → host path, listing shows host content |
| 5 refusals | PASS: `would shadow the Claude config dir` for exact and descendant targets; `contains ':'` for `/tmp/a:b`; relative `CLAUDE_CONFIG_DIR=cc` made absolute |
| 6 image parity | PASS: one rebuild from the template change (`3af11b592350`); switching `CLAUDE_CONFIG_DIR` added no tag |
| 7 nested | PASS: project at `$HOME`, `touch ~/.claude/.w` ok |
| 8 macOS | NOT RUN: no darwin host; `GOOS=darwin go vet` clean |
| guard (US3 AS2) | PASS: `-e CLAUDE_CONFIG_DIR=/home/kekkai/.claude` → no `ln`, dir stays the mountpoint; `ln -sfn` rerun idempotent; image has no baked `/home/kekkai/.claude` |
