# Research: Quiet Interactive Build Progress Without Custom Apt Repos

## R1. Root cause confirmation

**Finding**: BuildKit selects its progress renderer (`auto`) by testing whether its output stream is a terminal. Since specs/020 T009, `docker.BuildImage` sets `cmd.Stdout`/`cmd.Stderr` to `io.MultiWriter(os.Std*, &buf)` — an `exec.Cmd` writer that is not an `*os.File`, so the child gets a pipe, TTY detection fails, and BuildKit falls back to plain progress on every build. Confirmed in the field (user report, 2026-08-20) and consistent with pre-020 behavior where `cmd.Stdout = os.Stdout` passed the real fd through.

## R2. Gate mechanism: one function, one `capture` parameter

**Decision**: `BuildImage(tag, contextDir string, labels map[string]string, verbose, capture bool) (string, error)`. `capture=false`: `cmd.Stdout = os.Stdout`, `cmd.Stderr = os.Stderr` (fds inherited, BuildKit auto-detects the terminal), return `""`. `capture=true`: existing `MultiWriter` tee, return the buffer. Call site: `buildImage` passes `len(aptRepos) > 0`.

**Rationale**: The docker package stays apt-ignorant (020 R7 principle): it receives a plumbing instruction, not repo knowledge. One boolean beats a second exported function (`BuildImageCaptured`) — single code path for args/labels/verbose, no duplication.

**Alternatives considered**: separate function — duplicated arg assembly; passing the repos slice into docker — leaks runtime config into the docker package, rejected in 020 already.

## R3. TTY detection stays docker's job

**Decision**: kekkai performs no terminal detection for builds. With `capture=false`, docker inherits the real stdio and applies its own `auto` progress logic (compact on TTY, plain on pipe). With `capture=true`, plain rendering is the accepted cost of capture (spec US2 scenario 3).

**Rationale**: Matches the spec assumption verbatim; adding isatty logic in kekkai would duplicate docker's and can only disagree with it. Non-interactive invocations (CI, redirects) therefore behave exactly as if docker were run by hand — the pre-020 property.

**Alternatives considered (recorded in spec Assumptions, rejected)**: pty allocation to keep compact rendering while capturing — needs non-stdlib machinery (constitution III); `--progress=tty` through the pipe — fills the capture with ANSI control codes and the renderer's line truncation can cut off the exact `NO_PUBKEY ...` text the hint scans for.

## R4. Documentation impact: none

**Decision**: No SPECIFICATION.md amendment. Verified: §6.3's hint clause already reads "on a failed build with `apt_repos` configured whose output carries an apt signature-failure marker …" — true in both modes; no spec text anywhere describes progress rendering or output capture. The 020 contract file (`specs/020-apt-repos/contracts/image-render.md`) said "writes build output to the terminal as today **and** captures it"; this feature's contracts/build-output.md supersedes that sentence's *when*, cross-referenced rather than edited (specs are point-in-time artifacts; SPECIFICATION.md is the living source of truth and needs nothing).

**Rationale**: Constitution I requires spec amendment for design changes; the externally observable design (hint behavior, §6.3) is unchanged — only an internal rendering regression is fixed.

## R5. E2e observation of renderer mode

**Decision**: Drive `kekkai up` through a pseudo-TTY (`script -qec … /dev/null`) so docker sees a terminal, capture the session transcript, and classify: compact mode ⇔ transcript contains the `[+] Building` header and lacks per-step `#N [internal] load build definition` plain lines; plain mode ⇔ the step lines are present. A build only needs to run a few seconds to emit its header — scenarios kill it after the markers appear (`timeout`), no full build required except the 020 S5 hint re-run (which fails fast at `apt-get update`, ~60–90 s).

**Rationale**: The renderer choice is emitted immediately; validating it needs the terminal illusion (`script`), not build completion. This keeps the whole quickstart at roughly one short failing build plus three sub-minute observations.

## R6. `--verbose` interaction

**Decision**: Unchanged code path: verbose appends `--progress=plain` regardless of `capture`. In quiet mode docker then prints plain to the inherited terminal; in capture mode identical to today.

**Rationale**: FR-004; the flag already means "show me everything" and composes orthogonally with the writer choice.
