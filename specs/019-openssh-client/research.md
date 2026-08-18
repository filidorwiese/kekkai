# Research: OpenSSH Client in Sandbox Image

## R1: Conditional (`ssh_agent: true` only) vs unconditional bake

- **Decision**: Unconditional — always in `builtinAptPackages`.
- **Rationale**: Constitution constraint: "Image hash derives from bake-time inputs only (§6.1); runtime config MUST never trigger a rebuild." `git.ssh_agent` is runtime config; keying the package on it would make the image differ per config and rebuild on toggle. Existing precedent in `internal/runtime/up.go:24`: "jq/aggregate stay baked even though only the allow_github path uses them: the image must be identical regardless of runtime config."
- **Alternatives considered**: Conditional install via template flag — rejected (violates §6.1, splits image identity); runtime `apt-get install` at container start — rejected (needs network/sudo at runtime, violates boundary principle and firewall bring-up ordering).

## R2: Which package provides SSH signing

- **Decision**: `openssh-client` (Debian trixie, from base repo — no new apt sources).
- **Rationale**: git SSH signing shells out to `ssh-keygen -Y sign` / `-Y verify`; agent-backed signing additionally needs the agent protocol client, both shipped in `openssh-client`. The same package provides `ssh` for `git@` remotes. Installed size ~5–6 MB plus small deps (`libfido2`, already-present `libssl`); comparable to existing convenience entries.
- **Alternatives considered**: `openssh-sftp-server`/`openssh-server` — not needed, server components rejected (boundary principle); copying a static `ssh-keygen` — pointless complexity vs one apt entry.

## R3: List placement and spec wording

- **Decision**: Add to the "convenience" tier? No — add under a git-support rationale next to `git`, `gh`; SPECIFICATION.md §5.1 gains `openssh-client` with note "(SSH signing + `git@` remotes via forwarded agent, §5.2)".
- **Rationale**: It is functionally required for a documented feature (`ssh_agent`), not mere convenience; grouping with git tooling keeps the list self-explanatory.
- **Alternatives considered**: silent append at end of list — works but loses the why; new "git" comment tier in the Go list mirrors the spec sentence at near-zero cost.

## R4: Duplicate user `apt_packages: [openssh-client]`

- **Decision**: No dedup code needed.
- **Rationale**: `apt-get install a a` is a no-op for the duplicate; the rendered Dockerfile would differ (hash change) but build fine — same behavior any existing duplicate of e.g. `git` already has. Adding dedup logic would be new surface for zero benefit (Minimal Surface).
- **Alternatives considered**: dedupe builtin+user lists before render — rejected: changes hashes for all users with any overlap today, more code, no failure mode being fixed.

## R5: Cache/migration impact

- **Decision**: None needed; natural rebuild.
- **Rationale**: Builtin list feeds the rendered Dockerfile → image hash → tag `kekkai:<hash>`, and `kekkai.config_hash` label. Both change once with the new binary; next `up` misses `docker image inspect` and rebuilds. Old images linger until user prunes — existing, accepted behavior for every package-list change (e.g. 014, 015).
