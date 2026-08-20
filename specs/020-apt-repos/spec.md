# Feature Specification: Custom Apt Repositories (`image.apt_repos`)

**Feature Branch**: `020-apt-repos`

**Created**: 2026-08-20

**Status**: Draft

**Input**: User description: "Custom apt repositories (image.apt_repos) — let a project declare additional apt repositories in `.kekkai.yaml` so `image.apt_packages` can install packages from third-party repos (e.g. Dart), with strict config validation, image-identity integration, and documentation updates."

## Clarifications

### Session 2026-08-20

- Q: Flat repositories (suite ending in `/`) — supported or rejected in v1? → A: Supported: `suite` may end with `/` (e.g. `./`), which denotes a flat repository; for such entries `components` must be empty/absent.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Install a package from a third-party apt repository (Priority: P1)

A developer's project needs a tool that is not in the default Debian repositories (e.g. the Dart SDK). They add an `image.apt_repos` entry with the repository's name, URL, suite, components, and signing-key URL to `.kekkai.yaml`, list the package in `image.apt_packages`, and run kekkai. The sandbox image is rebuilt with the repository registered and the package installed, ready to use inside the sandbox.

**Why this priority**: This is the core capability of the feature; without it nothing else has value. Today users simply cannot install packages that live outside Debian's default repos.

**Independent Test**: Add the Dart repository and `dart` package to a project config, run kekkai, and verify the `dart` command works inside the sandbox.

**Acceptance Scenarios**:

1. **Given** a config with an `apt_repos` entry (name, https url, suite, components, key_url) and a package from that repo in `apt_packages`, **When** the sandbox image is built, **Then** the build succeeds and the package is installed and usable inside the sandbox.
2. **Given** a repo entry with a `key_url`, **When** the image is built, **Then** the signing key is fetched at build time and the repository is pinned to that key, so packages are only accepted when signed by it.
3. **Given** a repo entry without `key_url` for a repository signed by a key the base system already trusts (e.g. Debian backports), **When** the image is built, **Then** package installation from that repository succeeds using the system keyrings.
4. **Given** a repo entry omitting `components`, **When** the image is built, **Then** the repository is registered with component `main`.
5. **Given** a repo entry for a flat repository (`suite` ending in `/`, e.g. `./`, no `components`), **When** the image is built, **Then** the repository is registered as a flat repository and its packages install.
6. **Given** `apt_repos` entries but the needed packages listed in `apt_packages`, **When** the image is built, **Then** packages resolve from the newly added repositories in that same build (no second build needed).

---

### User Story 2 - Invalid repository config is rejected before any build (Priority: P2)

A developer makes a mistake in an `apt_repos` entry: a typo'd field, an http URL, a duplicate name, or a value containing characters that could alter the image build. When they run kekkai, config validation fails immediately with a clear message naming the offending repo entry and field, before any image build or container work starts.

**Why this priority**: Every config field in `apt_repos` is interpolated into the image build, so it is an injection surface. Rejecting bad values at load time protects the build and gives fast, actionable feedback; it must ship together with P1 but is testable on its own.

**Independent Test**: Feed configs with each class of invalid value and verify each is rejected at config load with a message naming the repo and field, and that no build is attempted.

**Acceptance Scenarios**:

1. **Given** a repo entry whose `name` contains characters outside lowercase letters, digits, and hyphens, **When** the config is loaded, **Then** validation fails naming the entry and the `name` rule.
2. **Given** two repo entries with the same `name`, **When** the config is loaded, **Then** validation fails reporting the duplicate name.
3. **Given** a repo entry whose `url` or `key_url` is not https, **When** the config is loaded, **Then** validation fails naming the field and the https requirement.
4. **Given** any repo field containing whitespace, shell metacharacters, or apt option syntax (e.g. `[trusted=yes]`), **When** the config is loaded, **Then** validation fails; there is no way to express an unsigned/trusted repository.
5. **Given** a repo entry missing a required field (`name`, `url`, or `suite`), **When** the config is loaded, **Then** validation fails naming the missing field.
6. **Given** a repo entry whose `name` collides with the builtin GitHub CLI repository, **When** the config is loaded, **Then** validation fails with a message explaining the name is reserved.
7. **Given** a flat-repo entry (`suite` ending in `/`) that also sets `components`, **When** the config is loaded, **Then** validation fails explaining components must be omitted for flat repositories.
8. **Given** multiple invalid values in one config, **When** the config is loaded, **Then** all violations are reported in one pass (consistent with existing config validation behavior).

---

### User Story 3 - Repo changes trigger an image rebuild (Priority: P2)

A developer adds, edits, or removes an `apt_repos` entry in an existing project. On the next kekkai run, the tool detects that the image definition changed and rebuilds the sandbox image, so the running sandbox always matches the declared repositories.

**Why this priority**: Without this, edits to `apt_repos` would silently have no effect on an already-built image, breaking the feature's core promise. Same importance tier as validation.

**Independent Test**: Build an image, change only an `apt_repos` field, run kekkai again, and verify a rebuild is triggered; revert the change and verify the original image is reused.

**Acceptance Scenarios**:

1. **Given** a project with a built sandbox image, **When** any `apt_repos` field is added, changed, or removed, **Then** the image identity changes and the next run rebuilds the image.
2. **Given** a project with a built sandbox image, **When** the config is unchanged, **Then** the existing image is reused (no spurious rebuilds).

---

### User Story 4 - Helpful failure when a signing key is missing or wrong (Priority: P3)

A developer configures a repository but omits `key_url` (or points it at the wrong key) for a repo the system does not already trust. The image build fails with apt's signature error, and kekkai surfaces a hint pointing at the `key_url` of the offending repo entry so the developer knows what to fix.

**Why this priority**: Quality-of-life on top of P1; the build fails either way, this makes the failure diagnosable without apt expertise.

**Independent Test**: Configure a repo needing a key without its `key_url`, run a build, and verify the failure output includes a hint naming the repo entry and suggesting `key_url`.

**Acceptance Scenarios**:

1. **Given** a repo entry whose packages cannot be verified (apt reports a missing public key), **When** the image build fails, **Then** the error output includes a hint identifying the repo entry by name and pointing at its `key_url` field.

---

### Edge Cases

- `apt_repos` set but `apt_packages` empty: repositories are registered in the image; build still succeeds.
- `apt_repos` is an empty list: treated the same as absent; image identity equals the no-repos identity (no rebuild churn between `[]` and omitted).
- Repo URL or key URL unreachable at build time: image build fails with the underlying download error, consistent with existing build-time downloads (nvm, npm, GitHub CLI repo).
- Key at `key_url` served in either common key format (ASCII-armored or binary): both work.
- A user repo entry that duplicates the builtin GitHub CLI repository's name: rejected at validation (reserved name); a user repo pointing at the same URL as the builtin is allowed but pointless — no silent collision on filenames can occur since names are unique and the builtin name is reserved.
- `components` with multiple space-separated values: not expressible (whitespace is rejected); a single component string only in v1 (see Assumptions).
- Flat repository (`suite` ending in `/`) with `components` set: rejected at validation — components are meaningless for flat repos.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The project config MUST accept an optional `image.apt_repos` list. Each entry has fields: `name` (required), `url` (required), `suite` (required), `components` (optional, default `main` for non-flat repos), `key_url` (optional).
- **FR-002**: `name` MUST match `^[a-z0-9-]+$` and be unique within the list; it is used for keyring and sources filenames and in error messages. Names reserved by builtin repositories (the GitHub CLI repo) MUST be rejected.
- **FR-003**: `url` and `key_url` MUST be https URLs; any other scheme is rejected.
- **FR-004**: Every `apt_repos` field value MUST be rejected if it contains whitespace, shell metacharacters, or apt option syntax. There MUST be no way to express an unsigned-repository escape hatch (e.g. `trusted=yes`) through any field.
- **FR-005**: All `apt_repos` validation MUST run at config load, before any image or container work, and violations MUST be reported in the same single pass as other config errors.
- **FR-006**: When `key_url` is present, the signing key MUST be fetched at image build time and the repository pinned to that key, so only packages signed by it are accepted. When `key_url` is absent, the repository MUST rely on the keys the base system already trusts.
- **FR-007**: Declared repositories MUST be registered in the image before package installation, so packages in `image.apt_packages` resolve from them within the same build.
- **FR-008**: Any change to `apt_repos` (add/edit/remove entry or field) MUST change the image identity and trigger a rebuild on the next run; an unchanged config MUST NOT trigger a rebuild. An empty list MUST be equivalent to the field being absent.
- **FR-009**: The builtin GitHub CLI repository MUST remain builtin and unaffected by user repo entries.
- **FR-010**: When an image build fails because apt cannot verify a repository's signature (missing/unknown public key), the failure output MUST include a hint identifying the offending repo entry by name and pointing at its `key_url`.
- **FR-011**: The generated example config, README, and SPECIFICATION.md MUST document the new fields (spec-first: specification updated in the same change as the behavior).
- **FR-012**: Flat repositories MUST be supported: a `suite` ending in `/` (e.g. `./`) denotes a flat repository and is registered without components. For such entries, a set `components` MUST be rejected at config load. The trailing `/` (and `/` within a flat suite path) is exempt from the metacharacter rejection; all other validation rules apply unchanged.

### Key Entities

- **Apt repository entry**: A user-declared package source: `name` (identifier, filename-safe, unique), `url` (https base URL), `suite` (distribution string; trailing `/` denotes a flat repository), `components` (defaults to `main`; must be absent for flat repos), `key_url` (optional https key location). Lives under `image` in the project config; part of image identity.
- **Builtin repository**: The GitHub CLI repository kekkai bakes in unconditionally. Not configurable; its name is reserved against user entries.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can go from "package not available in Debian" to a working tool inside the sandbox by adding one repo entry and one package name to the config and running kekkai once — no manual image or container steps.
- **SC-002**: 100% of the invalid-value classes (bad name, duplicate name, non-https URL, injection characters, apt option syntax, reserved name, missing required field, components on a flat repo) are rejected at config load with a message naming the repo entry and field, before any build starts.
- **SC-003**: Editing any `apt_repos` value and re-running kekkai always yields a sandbox built with the new repository set; re-running with an unchanged config never rebuilds.
- **SC-004**: A signature-verification build failure includes a hint that names the offending repo entry, so the user can fix `key_url` without reading apt internals.
- **SC-005**: Runtime egress firewall behavior is unchanged: no new destinations are opened at runtime by declaring repositories.

## Assumptions

- The config author is trusted. Validation exists to prevent accidental or structural injection into the image build, not to restrict what the user may install (consistent with the existing security model: build-time downloads happen on the host network; the runtime firewall is untouched).
- `components` is a single component string (defaults to `main`). Multi-component repos are assumed rare for this use case; the whitespace rejection rule makes multiple components inexpressible in v1. If needed later, a list form can be added without breaking existing configs.
- Architecture selection for repositories follows the image's own architecture (as apt defaults do); no per-repo architecture field.
- Signing keys are fetched over https at build time only; keys are not stored in or referenced from the project config beyond `key_url`.
- Out of scope (per feature description): raw single-line deb entries, deb822 `.sources` output, arbitrary setup/RUN escape hatches, custom base images.
- Constitution alignment: image hash derives from bake-time inputs only — `apt_repos` is a bake-time input (Constraint compliance); no change to the builtin destination set or firewall (Principle II); one new config key justified by the spec (Principle III); validation end-to-end against a real build (Principle IV).
