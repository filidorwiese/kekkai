# Contract: Builtin Python Runtime

**Feature**: 024-python-builtin | **Date**: 2026-08-24

The user-facing contract every kekkai sandbox image honors after this feature.

## Package presence (FR-001, FR-003)

In every sandbox, regardless of project config, from every execution path (interactive shell, `kekkai exec`, subprocesses — no profile sourcing required):

| Command | Contract |
|---------|----------|
| `python3` | resolves; Python 3.13.x (trixie); stdlib complete (`json`, `csv`, `sqlite3`, `urllib`, ...) |
| `pip` / `pip3` | resolves; pip ≥ 25 |
| `python3 -m venv <dir>` | creates a working venv whose own `pip` functions |

All three arrive via Debian trixie main (`python3`, `python3-venv`, `python3-pip`), installed with `--no-install-recommends` in the existing single apt layer. No compilers, no `python3-dev` (spec non-goal).

## PEP 668 guard (FR-002)

- `/etc/pip.conf` exists in every image with exactly:

  ```ini
  [global]
  break-system-packages = true
  ```

- Consequence: plain `pip install <pkg>` never fails with `externally-managed-environment` — as root it installs to system site-packages; as the sandbox user it auto-falls-back to a `--user` install under `~/.local`.
- Inside venvs the option is inert; venv behavior is identical to an unconfigured system.
- Precedence: `/etc/pip.conf` is pip's lowest tier; user/env/CLI pip config can still override at runtime. Kekkai itself never writes any higher-tier pip config.

## Network egress posture (FR-006, FR-007 — clarification 2026-08-24)

- The §5.4 always-allowed firewall set is UNCHANGED. `pypi.org` / `files.pythonhosted.org` are NOT baked.
- `pip install` reaching the package index requires the project to opt in:

  ```yaml
  network:
    allowed_domains: [pypi.org, files.pythonhosted.org]
  ```

- Without the opt-in, `pip install` fails at the firewall (connection refused/timeout) — never with a PEP 668 error. README documents this; posture matches runtime `npm install` (npm registry likewise not baked).

## Identity (FR-005)

- The three packages and the pip.conf template line are bake-time inputs: upgrading kekkai changes the rendered Dockerfile → new `ImageTag` → exactly one rebuild per project; unchanged configs then reuse the image indefinitely.
- `ConfigHash` changes via the extended package list, so the §6.2 offline fallback never serves a pre-upgrade image for the new definition.

## Security invariants (FR-006)

- No new sudoers entries, daemons, services, or firewall/verification changes; sandbox runtime behavior is byte-identical apart from the tools existing and the pip guard file.
