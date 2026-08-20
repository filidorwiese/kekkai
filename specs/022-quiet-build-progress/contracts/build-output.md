# Contract: build output plumbing and progress rendering

Scope: `internal/docker/cli.go` (`BuildImage`), `internal/runtime/up.go` (`buildImage` call site). Supersedes the *when-to-capture* sentence of `specs/020-apt-repos/contracts/image-render.md` ("writes build output to the terminal as today and captures it"); every other clause of the 020 hint contract stands unchanged.

## BuildImage modes

`BuildImage(tag, contextDir, labels, verbose, capture) (output string, err error)`

| Mode | Writers | Progress rendering | Return `output` |
|---|---|---|---|
| `capture=false` | child inherits `os.Stdout` / `os.Stderr` | docker's own `auto`: compact TTY UI on a terminal, docker's fallback otherwise — indistinguishable from running `docker build` by hand | `""` always |
| `capture=true` | `io.MultiWriter(os.Std*, buffer)` on both streams | plain (BuildKit sees a pipe) — accepted cost of capture | full captured stream, on success and failure |

Both modes: `verbose` appends `--progress=plain`; error text remains `docker build failed: …`; args/labels assembly identical.

## Call site

`buildImage` passes `capture = len(cfg.Image.AptRepos) > 0`. Consequences:

- Zero repos (incl. `[]`): quiet mode. The hint scan (`aptSignatureHint`) receives an empty repo slice and returns `""` — no behavior depends on the missing capture.
- ≥1 repo: capture mode; the specs/020 hint contract applies byte-for-byte (markers `NO_PUBKEY` / `is not signed` / `EXPKEYSIG` / `NODATA`, attribution by URL, exact wording, stderr after the build error, exit status unchanged).

## Guarantees

- No identity input changes: rendered Dockerfile, firewall script, `ImageTag`, `ConfigHash` all byte-identical to pre-fix — existing images reused, zero rebuilds (FR-005 / SC-003).
- No hint is ever printed without `apt_repos` — unchanged from 020, now also structurally true (nothing captured to scan).
- No new config keys or flags (FR-006); no terminal detection inside kekkai (research.md R3).
