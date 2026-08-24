# Feature Specification: Builtin Python Runtime

**Feature Branch**: `024-python-builtin`

**Created**: 2026-08-24

**Status**: Draft

**Input**: User description: "Add python3 to the builtin apt packages baked into the kekkai sandbox image: python3, python3-venv, python3-pip. Also bake /etc/pip.conf with break-system-packages = true so pip installs work frictionlessly in the disposable sandbox despite PEP 668. Rationale: Claude Code defaults to writing ad-hoc python scripts; matches specs/021 'near-universal agent operations, zero config required' bar."

## Clarifications

### Session 2026-08-24

- Q: Should any external network resource (package index) be baked into the always-allowed egress firewall set so pip installs work out of the box? → A: No baked destination. Document in user-facing docs that pip installs need `pypi.org` + `files.pythonhosted.org` in `network.allowed_domains` (same opt-in posture as the npm registry for runtime `npm install`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent runs ad-hoc Python scripts out of the box (Priority: P1)

An autonomous agent inside a fresh sandbox writes a throwaway Python script — parsing JSON, munging CSV data, probing an HTTP API, querying SQLite — and runs it with `python3` immediately, using the standard library, without the project author having configured anything.

**Why this priority**: This is the core gap. Ad-hoc Python scripting is one of the most common agent operations (on par with the search and archive operations that motivated the specs/021 general-tooling tier), and today every such attempt fails with "command not found" until the user discovers the missing package and adds it per-project via `image.apt_packages`.

**Independent Test**: Start a sandbox from a project with no config file and run a `python3` one-liner plus a small script exercising stdlib modules (`json`, `csv`, `sqlite3`, `urllib`).

**Acceptance Scenarios**:

1. **Given** a fresh sandbox with no project config, **When** the agent runs `python3 --version`, **Then** the interpreter resolves and reports its version.
2. **Given** a fresh sandbox, **When** the agent runs a script importing `json`, `csv`, `sqlite3`, and `urllib`, **Then** the script executes successfully.

---

### User Story 2 - Agent installs Python packages without PEP 668 friction (Priority: P2)

An agent needs a third-party library (e.g. `requests`) for a quick task. It runs `pip install <package>` and the install proceeds directly — no "externally-managed-environment" error, no retry with special flags. Agents that prefer isolation can instead create a virtual environment with `python3 -m venv` and it works on the first attempt.

**Why this priority**: Without this, the first `pip install` in every sandbox fails on the distribution's externally-managed-environment guard and the agent burns a turn recovering. The sandbox is disposable — the system-Python protection the guard exists for is irrelevant here. Lower than P1 because the agent does self-recover (the error message states the workaround) and stdlib covers most ad-hoc scripts.

**Independent Test**: In a fresh sandbox (with the package index reachable per project config), run `pip install` for a small package and import it; separately create a venv, activate it, and pip-install into it.

**Acceptance Scenarios**:

1. **Given** a fresh sandbox, **When** the agent runs `pip install` for a package, **Then** the install is not blocked by the externally-managed-environment guard.
2. **Given** a fresh sandbox, **When** the agent runs `python3 -m venv .venv`, **Then** the environment is created successfully with a working `pip` inside it.
3. **Given** a fresh sandbox whose project config does not allow the package index as a network destination, **When** the agent runs `pip install`, **Then** the install fails on network egress exactly as any other blocked destination would — never on the environment guard.

---

### User Story 3 - Existing projects upgrade cleanly (Priority: P2)

A user upgrades kekkai. On the next run, each of their projects rebuilds its sandbox image once (the builtin package set and baked configuration are part of the image definition); after that one rebuild, unchanged configs reuse the image exactly as before. Projects that already listed a Python package in `image.apt_packages` keep working without edits.

**Why this priority**: The feature must not degrade the existing no-churn guarantee: identity semantics are a standing promise of the tool.

**Independent Test**: Build an image with the previous release, upgrade, run again and observe exactly one rebuild; run a third time and observe reuse. Repeat with a config that lists `python3` in `apt_packages`.

**Acceptance Scenarios**:

1. **Given** a project with an image built by the previous release, **When** the upgraded kekkai runs, **Then** the image rebuilds once and the sandbox starts normally.
2. **Given** the rebuilt image, **When** kekkai runs again with an unchanged config, **Then** the image is reused (no further rebuilds).
3. **Given** a config whose `apt_packages` already lists `python3` or `python3-pip`, **When** the image builds, **Then** the build succeeds and the tools are present (duplication is harmless).

---

### Edge Cases

- A project's `apt_packages` duplicates a new builtin: the package manager treats the repeated name as a no-op; no error, no behavior change required from the user.
- The offline-fallback image lookup keys on the bake inputs: after upgrade, pre-upgrade images no longer match the new package set — expected; the fallback must never serve an image missing the Python runtime or the baked pip configuration.
- Package installs need network egress to the package index: this feature deliberately does NOT open any new network destination (clarified 2026-08-24). `pip install` against a disallowed index fails at the firewall like any other blocked destination; users opt in per project by adding `pypi.org` and `files.pythonhosted.org` to `network.allowed_domains`, and the user-facing documentation states exactly that. The baked pip configuration only removes the environment guard, never a network control.
- A user's project bakes its own pip configuration via image customization: the builtin baked configuration must not silently clobber a more specific user-provided one at the same path; last-writer semantics must be deterministic and documented in the implementation plan.
- Virtual environments: the environment guard applies only to the system interpreter; venvs created in the sandbox behave identically before and after this feature (the baked configuration is compatible with venv use).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The builtin package set baked into every sandbox image MUST additionally include: `python3` (interpreter), `python3-venv` (virtual-environment support — without it environment creation fails on the base distribution), `python3-pip` (package installer).
- **FR-002**: Every sandbox image MUST bake a system-wide pip configuration that disables the externally-managed-environment guard (PEP 668), so that a plain `pip install` in the sandbox proceeds without extra flags or user configuration.
- **FR-003**: `python3`, `pip` (and/or `pip3`), and `python3 -m venv` MUST be invocable in the sandbox from every execution path (interactive shell, one-off exec, subprocesses).
- **FR-004**: The builtin set remains a code constant: this feature adds no config keys, no schema change, and no user-visible toggles. User `apt_packages` continues to append to the builtin set unchanged.
- **FR-005**: The extended package set and the baked pip configuration are bake-time inputs: they MUST enter the image identity (triggering exactly one rebuild per project on upgrade) and the config-hash used by the offline fallback. After the one-time rebuild, an unchanged config MUST NOT rebuild.
- **FR-006**: Runtime security behavior MUST be unchanged: no new sudoers entries, no new builtin network destinations (the package index is NOT added to the firewall's builtin set), no daemons or services, firewall configuration and verification untouched.
- **FR-007**: User-facing documentation (README) MUST state that runtime `pip install` requires the project to allow `pypi.org` and `files.pythonhosted.org` via `network.allowed_domains` — the always-allowed firewall set is unchanged, matching the npm-registry posture for runtime `npm install`.
- **FR-008**: SPECIFICATION.md's builtin package list (and image bake description, for the pip configuration) MUST be amended in the same change as the behavior (spec-first). The specs/021 non-goal note that grouped `python3` with the native build toolchain is superseded in the narrow sense: interpreter + pip only; compilers stay out of scope.

### Key Entities

- **Builtin package set**: The code-constant list of packages baked into every sandbox image regardless of config. This feature extends the general-tooling tier (specs/021) with a scripting-runtime entry.
- **Baked pip configuration**: A fixed, code-constant configuration file placed in the image at bake time that relaxes the externally-managed-environment guard. Part of the image definition, invisible to runtime config.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a sandbox started from a project with no config file, `python3` runs a stdlib-only script successfully on the first attempt.
- **SC-002**: In a fresh sandbox with the package index allowed by project config, a plain `pip install` of a small package succeeds on the first attempt with zero PEP 668-related errors or retries.
- **SC-003**: `python3 -m venv` creates a working virtual environment (including its own `pip`) on the first attempt.
- **SC-004**: Upgrading kekkai causes exactly one image rebuild per project config; subsequent runs with unchanged configs reuse the image 100% of the time.
- **SC-005**: Startup firewall verification probes pass unchanged; no new network destinations are reachable from the sandbox.
- **SC-006**: The sandbox image grows by less than 120 MB from the added packages and configuration.

## Assumptions

- Interpreter + venv + pip are daemon-free and add no network egress paths by themselves; the container remains the security boundary (Principle II untouched — this changes image contents, not the boundary). Package-index reachability stays a per-project config decision.
- Disabling the PEP 668 guard is acceptable because the sandbox is disposable: the guard protects a long-lived system Python from corruption, a concern that does not apply to a throwaway container. Venv-based workflows remain fully available for agents that prefer isolation.
- Explicit non-goals: `python3-dev`, `build-essential`/compilers (native-extension toolchain — the heavy half of the specs/021 rejection stands; pure-Python wheels and prebuilt binary wheels cover the common cases), `pipx` (redundant given the relaxed guard), preinstalled third-party libraries such as `requests` (scope creep; stdlib plus on-demand pip covers ad-hoc scripting), any change to builtin firewall destinations (constitution Principle II requires a spec amendment for that — deliberately not made here).
- A one-time rebuild for every user on upgrade is acceptable and expected; release notes should mention it.
- Constitution alignment: bake-time-inputs-only image hash respected (FR-005); no change to the builtin network destination set or firewall (Principle II, FR-006); no new config surface (Principle III, FR-004); validation end-to-end against a real docker daemon (Principle IV).
