# Quickstart Validation: Builtin Python Runtime

**Feature**: 024-python-builtin | **Date**: 2026-08-24

End-to-end validation against a real docker daemon (constitution IV). Contract details: [contracts/python-runtime.md](contracts/python-runtime.md).

## Prerequisites

- Docker daemon running; `go build ./cmd/kekkai` succeeds
- A scratch project dir (`mkdir -p /tmp/kekkai-py-test && cd /tmp/kekkai-py-test`)
- Interactive `up` needs the pseudo-TTY trick from prior validations (`script -qec "kekkai up" /dev/null`) or run `up` in one terminal and `kekkai exec` from another

## Scenario 1 — Interpreter + stdlib, zero config (User Story 1, SC-001)

No `.kekkai.yaml` in the project. Start the sandbox, then:

```sh
kekkai exec -- python3 --version
kekkai exec -- python3 -c 'import json,csv,sqlite3,urllib.request; print("stdlib ok")'
kekkai exec -- pip --version
```

**Expected**: Python 3.13.x; `stdlib ok`; pip ≥ 25. All resolve without PATH tricks.

## Scenario 2 — venv works (SC-003)

```sh
kekkai exec -- python3 -m venv /tmp/v
kekkai exec -- /tmp/v/bin/pip --version
```

**Expected**: venv created; in-venv pip reports its version.

## Scenario 3 — pip guard off, firewall still governs (User Story 2, SC-002, SC-005)

Without any network config:

```sh
kekkai exec -- pip install six
```

**Expected**: FAILS with a network/connection error (firewall block) — and NOT with `externally-managed-environment`. This is the guard-vs-firewall separation.

Then add to `.kekkai.yaml`:

```yaml
network:
  allowed_domains: [pypi.org, files.pythonhosted.org]
```

Restart the sandbox (`kekkai down && up` — runtime config, no rebuild expected) and:

```sh
kekkai exec -- pip install six
kekkai exec -- python3 -c 'import six; print("six", six.__version__)'
```

**Expected**: install succeeds on first attempt with zero PEP 668 errors (non-root `--user` fallback into `~/.local`); import works. Startup firewall verification probes passed unchanged both runs.

## Scenario 4 — One rebuild on upgrade, then reuse (User Story 3, SC-004)

```sh
# with the PREVIOUS kekkai binary: build an image, note `docker images kekkai`
# with the NEW binary:
kekkai up        # expect: one image build
kekkai down && kekkai up   # expect: no rebuild, image reused
```

Also: add `apt_packages: [python3]` to a config — build succeeds, duplication harmless.

## Scenario 5 — Image size budget (SC-006)

Compare `docker images` sizes of pre- and post-feature images for the same config.

**Expected**: growth < 120 MB (measured baseline: 41 MB installed footprint).

## Cleanup

```sh
kekkai down; rm -rf /tmp/kekkai-py-test /tmp/v
```
