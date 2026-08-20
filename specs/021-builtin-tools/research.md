# Research: Builtin Archive, File-Inspection, and Search Tools

All Technical Context unknowns resolved. Findings verified against a real `debian:trixie` container (2026-08-20).

## R1. Package availability and size on debian:trixie

**Decision**: Install exactly `unzip zip xz-utils zstd bzip2 file ripgrep fd-find rsync` from trixie main; no backports, no third-party repos.

**Rationale**: All nine install cleanly in one `apt-get install` on trixie (verified). Installed sizes (KB, incl. notable transitive deps): libmagic-mgc 10154, ripgrep 5278, fd-find 3492, zstd 2320, xz-utils 1424, rsync 830, zip 627, unzip 387, libpopt0 245, libmagic1t64 244, bzip2 113, file 79 — **~25 MB total**, comfortably within the spec's 50 MB bound (SC-005). None ship daemons, services, or setuid binaries relevant to the sandbox.

**Alternatives considered**: none needed — first-party Debian packages are the only sensible source for builtins (third-party repos are user territory via `image.apt_repos`, specs/020).

## R2. Canonical `fd` name (FR-002)

**Decision**: Add `ln -s /usr/bin/fdfind /usr/local/bin/fd` to the existing root-owned symlink RUN block in `embed/Dockerfile.tmpl` (the block that already links `node`/`npm`/`npx`/`claude` into `/usr/local/bin`).

**Rationale**: Debian renames fd's binary to `fdfind` (upstream name clash with an older package); verified: after install, `fd` is absent from PATH, `fdfind` is at `/usr/bin/fdfind`, and the package also ships the upstream-named binary at `/usr/lib/cargo/bin/fd` (not on PATH). Symlinking to `/usr/bin/fdfind` — the path Debian documents and guarantees — into `/usr/local/bin` follows the image's established exec-path pattern exactly (§6.3) and works from every exec path. All other eight tools already use their canonical names (`rg` included), needing nothing.

**Alternatives considered**: symlink to `/usr/lib/cargo/bin/fd` — same binary, but the cargo path is a Debian packaging implementation detail with weaker stability guarantees; adding `/usr/lib/cargo/bin` to PATH — broader than needed; documenting `fdfind` as-is — rejected by FR-002 (agents type `fd`; a wrong-name failure is exactly the confusing mid-task breakage this feature removes).

## R3. Placement in `builtinAptPackages` (feature description decision 2)

**Decision**: New comment-labeled group `// general tooling (specs/021): archives, file inspection, search, file transfer` appended after the existing "convenience" group, listing the nine packages in the spec's order.

**Rationale**: The constant is organized as purpose-grouped lines with comments (firewall/lifecycle, nvm dependency, subcommands, git-over-ssh, convenience); a distinct group keeps the provenance visible and diffs reviewable. Order within the list is cosmetic — the serialized list enters `ConfigHash` as-is either way, and any ordering produces the same one-time rebuild.

**Alternatives considered**: folding into the existing "convenience" group — loses the spec back-reference; alphabetizing the whole constant — churns unrelated lines and adds nothing.

## R4. Identity and rebuild mechanics (FR-004)

**Decision**: No identity code changes. The extended constant flows into both identity inputs automatically: `builtinAptPackages` is concatenated with user `apt_packages` and (a) rendered into the Dockerfile's install line → `ImageTag` changes → one-time rebuild per config; (b) passed to `ConfigHash` → the offline-fallback label changes → pre-upgrade images can never be served for the new definition.

**Rationale**: This is precisely the specs/019 (openssh-client) precedent — that feature also only edited the constant and SPECIFICATION.md. The template edit (R2 symlink line) additionally changes every rendered Dockerfile, which is subsumed by the same one-time rebuild. After the rebuild, unchanged configs hash identically → reuse (US3).

**Alternatives considered**: none — the pipeline exists for exactly this.

## R5. Duplicate package names in user `apt_packages` (US3 scenario 3)

**Decision**: No handling needed; document as harmless.

**Rationale**: The install line becomes `apt-get install ... ripgrep ... ripgrep` when a user already lists a new builtin; apt treats repeated names in one invocation as a single request (verified behavior of apt-get). No dedup code, matching how the existing builtins already behave (a user listing `git` today is equally harmless).

**Alternatives considered**: deduplicating the concatenated list in Go — added code for a non-problem, and it would change the rendered Dockerfile/hashes for configs that currently contain duplicates, causing exactly the churn the spec forbids.

## R6. arm64 parity

**Decision**: No special handling; all nine packages exist in trixie main for amd64 and arm64 (Debian arch:any builds; ripgrep and fd-find are packaged for both).

**Rationale**: The image is built for the daemon's native platform; the builtin set must install on both supported linux arches. Trixie ships both; no arch-conditional logic in the template today and none needed.

## R7. Documentation targets (FR-006)

**Decision**: SPECIFICATION.md only: §5.1 gains the nine packages in the builtin list with the group rationale; §6.3's exec-path bullet gains the `fd` symlink alongside `node/npm/npx/claude`. README is not amended — it does not enumerate builtin packages today (it documents config, and this feature adds none). Release-notes mention of the one-time rebuild goes in the release/commit description, not a repo file (no changelog file exists).
