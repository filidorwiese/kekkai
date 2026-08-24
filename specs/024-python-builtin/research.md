# Research: Builtin Python Runtime

**Feature**: 024-python-builtin | **Date**: 2026-08-24

All findings verified live against `debian:trixie` (the image base) on amd64, 2026-08-24.

## R1: Package selection

**Decision**: `python3`, `python3-venv`, `python3-pip` — exactly the three from the feature description.

**Rationale**:
- `python3` → Python 3.13.5 interpreter + full stdlib (`json`, `csv`, `sqlite3`, `urllib` verified importable).
- `python3-venv` → without it `python3 -m venv` fails on Debian (ensurepip is split out of the interpreter package). Verified: venv creation produces a working in-venv pip 25.1.1.
- `python3-pip` → pip 25.1.1. Needed for system/user installs; venv-only would leave the P2 frictionless-install story unmet.

**Alternatives considered**:
- `python3` alone: venv broken, no pip — fails User Story 2.
- `python3` + `python3-venv` only: pip exists only inside venvs; plain `pip` not on PATH — agents' first `pip install` fails harder than today's PEP 668 error.
- `python3-dev`, `build-essential`: native-extension toolchain; the heavy half of the specs/021 rejection stands (spec non-goal). Prebuilt wheels cover the common cases.
- `pipx`: redundant once the guard is off.

## R2: Install cost

**Decision**: accept 23 packages / 12.4 MB download / 41.0 MB installed (`apt-get install --no-install-recommends`, trixie, amd64).

**Rationale**: well inside SC-006's 120 MB. The "~350 MB" figure that got python rejected in specs/021 scoping was `python3-pip`'s *Recommends* (`build-essential`, `python3-dev`); the Dockerfile already installs with `--no-install-recommends`, so that cost never materializes. Without the flag the same install is 98 packages.

**Alternatives considered**: none needed — measurement settles it.

## R3: PEP 668 bypass mechanism

**Decision**: bake `/etc/pip.conf` containing `[global]` / `break-system-packages = true`, written by one `printf` line appended to the existing first root RUN block in `embed/Dockerfile.tmpl` (same block that installs apt packages; root context, no new RUN layer needed... place it in that block's command chain).

**Rationale**, all container-verified:
- Root `pip install six` on system python: proceeds, no externally-managed-environment error.
- Non-root uid-1000 user (the sandbox case): system site-packages unwritable → pip auto-falls-back to `--user` install into `~/.local` — PEP 668 normally blocks that path too; with the baked conf it succeeds and the module imports.
- `pip config list` shows `global.break-system-packages='true'`; other pip commands (`download`, `config`) unaffected by the `[global]` placement.
- Inside venvs: no `EXTERNALLY-MANAGED` marker exists, the option is inert — venv workflows unchanged.
- `/etc/pip.conf` is pip's documented global-config path, lowest precedence: a user/project could still override per-run via `PIP_CONFIG_FILE` or `~/.config/pip/pip.conf` at runtime — deterministic last-writer semantics for the spec's edge case (kekkai offers no image-level file customization, so no bake-time clobber scenario exists).

**Alternatives considered**:
- `ENV PIP_BREAK_SYSTEM_PACKAGES=1` in the Dockerfile: works, but docker `ENV` leaks into `docker inspect`/`env` output and §5.3 builtin-env semantics say user `env` entries override builtins last-value-wins — a user env block could silently re-enable the guard. The file is bake-time-only and outside the env pipeline.
- Deleting the `EXTERNALLY-MANAGED` marker file: fragile (path embeds the python minor version, breaks on trixie point upgrades), and semantically a distro-file mutation rather than a configuration.

## R4: Image identity / rebuild semantics

**Decision**: no identity code changes.

**Rationale**:
- `ImageTag` = sha256(rendered Dockerfile + init-firewall.sh): both the 3 new packages (rendered into the apt line) and the new pip.conf template line change the rendered text → exactly one rebuild per project on upgrade, then cache hits (FR-005).
- `ConfigHash` (offline-fallback key) includes `builtin+user apt_packages` → the package additions change it, so the fallback can never serve a pre-upgrade image lacking python/pip.conf. Note: the pip.conf line itself is NOT a ConfigHash input (template text never is — same situation as every prior template edit, e.g. specs/021's fd symlink); correctness is carried by the simultaneous package change in the same release. Documented here as the standing pattern for template-only edits riding a package change.

**Alternatives considered**: adding rendered-template text to ConfigHash — rejected, changes established identity semantics for zero gain in this feature.

## R5: Network egress for pip (clarification 2026-08-24)

**Decision**: no firewall change. README documents `pypi.org` + `files.pythonhosted.org` as the `network.allowed_domains` entries pip needs; §5.4 always-allowed set untouched.

**Rationale**: spec clarification session (option A). Consistent with the npm posture: runtime `npm install` also requires user opt-in; §5.4 is deliberately "nothing else — not npm". Principle II: new destinations open via user config only. Baking PyPI would open arbitrary-code egress for every sandbox including ones never running python, and `files.pythonhosted.org` is a Fastly CDN whose resolve-once-at-start ipset semantics are brittle anyway.

**Alternatives considered**: bake into §5.4 (rejected: boundary widening + constitution amendment); `network.allow_pypi` toggle (rejected: new config surface, Principle III).

## R6: PATH note for `--user` installs

**Decision**: no change; document nothing.

**Rationale**: non-root `pip install` lands console scripts in `~/.local/bin`, which Debian's default `.profile` adds to PATH only for login shells. Module imports (the dominant agent use) work regardless; agents needing a script binary can invoke `python3 -m <tool>` or add the dir themselves. Baking a PATH edit for this corner exceeds the feature's minimal-surface bar.

**Alternatives considered**: append `~/.local/bin` to the image `ENV PATH` — deferred; can ride a future feature if it proves a real friction point.
