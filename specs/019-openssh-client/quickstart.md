# Quickstart Validation: OpenSSH Client in Sandbox Image

Per constitution IV: end-to-end against a real docker daemon. See [contracts/package-set.md](contracts/package-set.md) for the full check matrix.

## Prerequisites

- Host: docker, git, an ssh-agent running with a key loaded (`ssh-add -l` non-empty).
- Host git configured for SSH signing:
  `git config --global gpg.format ssh && git config --global user.signingkey 'key::<pubkey>' && git config --global commit.gpgsign true`
- `~/.config/git/allowed_signers` containing that key.
- Test project with `.kekkai.yaml`: `git: {enabled: true, ssh_agent: true}`.

## Steps

1. Build binary: `task build` (or `go build ./cmd/kekkai`).
2. `./kekkai up` in the test project — expect one image rebuild (new package set), firewall probes pass.
   - Note (from memory/e2e notes): `up` needs a pseudo-TTY when driven non-interactively — use `script -qec './kekkai up' /dev/null` style invocation.
3. Package presence (works even with `ssh_agent: false`):
   `kekkai exec 'command -v ssh ssh-keygen ssh-add'` → three paths.
4. Agent forwarding: `kekkai exec 'ssh-add -l'` → host keys listed.
5. Signed commit inside sandbox:
   `kekkai exec 'cd /workspace && git commit --allow-empty -m sign-test'` → succeeds.
6. Verify: `kekkai exec 'cd /workspace && git log --show-signature -1'` → "Good \"git\" signature".
7. No-rebuild check: flip `ssh_agent` to `false`, run `up` again → no image build triggered; flip back.
8. Duplicate tolerance: add `image: {apt_packages: [openssh-client]}`, run `up` → image (re)builds successfully, no apt error.

## Validation results (2026-08-18, linux/amd64, docker 29.7.2)

All steps pass. Notes:

- Step 3: `command -v` in dash prints only its first hit — loop per binary; all three present (`/usr/bin/ssh{,‑keygen,‑add}`).
- Steps 4–6: agent keys listed via `/ssh-agent`; empty commit signed and verified "Good \"git\" signature" with zero in-sandbox config.
- Step (US2): `git ls-remote git@github.com:filidorwiese/kekkai.git` authenticated via agent (needed `StrictHostKeyChecking=accept-new` — no known_hosts in image; normal first-connect behavior, not a regression).
- Step 7: `ssh_agent` flips both ways → zero build activity.
- Step 8: duplicate `openssh-client` in `apt_packages` → apt layer builds clean.

## Expected outcomes

- Steps 3–6 pass on first attempt with zero in-sandbox git/ssh configuration.
- Step 7 shows zero docker build activity.
- SPECIFICATION.md §5.1 amended in the same commit as the code change.
