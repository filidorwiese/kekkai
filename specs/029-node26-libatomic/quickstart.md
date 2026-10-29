# Quickstart: validate Node 26 LTS in the sandbox image

End-to-end against the real daemon (constitution IV). Linux host; macOS (Docker Desktop) deferred to release validation as in prior features.

## Prerequisites

- Docker running, network access to nodejs.org, registry.npmjs.org, deb.debian.org.
- Build the binary: `task build` (→ `/tmp/kekkai`, version `v0.0.0-dev`).
- A scratch project: `mkdir -p /tmp/k29 && cd /tmp/k29`.
- Clear state: `kekkai prune --yes` (or `docker rmi $(docker images -q 'kekkai:*')`).

## 1. Control: reproduce the defect with the old binary (optional)

With the pre-029 binary and no cached image, `kekkai up` in `/tmp/k29` fails at the claude install layer with `npm: command not found`, preceded in the nvm layer by `WARNING: Version 'none' does not exist`.

## 2. Default selector builds (US1)

```sh
cd /tmp/k29 && /tmp/kekkai up
```

Expected: one stderr warning about missing config, image build completes, startup line `kekkai sandbox: node v26.x.y, claude <ver>`, firewall probes pass, claude starts. Exit claude.

Then, with the sandbox running in another terminal:

```sh
/tmp/kekkai exec node -v     # v26.x.y
/tmp/kekkai exec npm -v      # 11.x
/tmp/kekkai exec claude --version
```

Build log check: the nvm layer shows `default -> v26.x.y` (not `none`).

## 3. Explicit 26 selectors (US2)

```sh
printf 'image:\n  node_version: "26"\n' > .kekkai.yaml && /tmp/kekkai up
printf 'image:\n  node_version: "26.13.0"\n' > .kekkai.yaml && /tmp/kekkai up
```

Expected: both build, startup lines report 26.x and exactly v26.13.0 respectively.

## 4. Earlier selectors still work, rebuild exactly once (US3)

```sh
printf 'image:\n  node_version: "24"\n' > .kekkai.yaml && /tmp/kekkai up   # builds, node v24.x
/tmp/kekkai up                                                              # no "building image" line
printf 'image:\n  node_version: "22"\n' > .kekkai.yaml && /tmp/kekkai up   # builds, node v22.x
```

## 5. Hash invalidation (FR-005)

```sh
docker images --filter 'reference=kekkai:*' --format '{{.Tag}} {{.Labels}}'
```

Expected: tags and `kekkai.config_hash` labels differ from any image built by the previous release for the same config. Simulate the offline fallback by blocking registry.npmjs.org (or `KEKKAI`-independent: disconnect) with only a pre-029 image present → `kekkai up` errors with "no existing kekkai image matches this config", never reuses the old one.

## 6. No new egress (FR-006)

With a sandbox running: `/tmp/kekkai traffic` in a second terminal for a minute of normal use shows only the destinations seen before this change. Firewall §9.6 probes pass at startup (visible in the startup output).

## 7. Duplicate user entry (edge case)

```sh
printf 'image:\n  apt_packages: [libatomic1]\n' > .kekkai.yaml && /tmp/kekkai up
```

Expected: builds and runs.

## 8. Spec and build hygiene

- `go build ./... && go vet ./...` clean.
- `SPECIFICATION.md` §5.1 contains the amendment from [contracts/builtin-packages.md](contracts/builtin-packages.md).
- Cleanup: `rm -rf /tmp/k29`; `kekkai prune --yes`.
