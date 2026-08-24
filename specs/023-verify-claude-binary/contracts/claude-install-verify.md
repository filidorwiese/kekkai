# Contract: Claude Install Verification Step

Governs the claude install RUN step in `embed/Dockerfile.tmpl` (replaces the bare `RUN npm install -g @anthropic-ai/claude-code@{{.ClaudeVersion}}`). Referenced from SPECIFICATION.md §6.3.

## Rendered step shape

```dockerfile
RUN npm install -g @anthropic-ai/claude-code@{{.ClaudeVersion}} \
 && claude --version \
 || { echo "kekkai: claude native binary missing after install, retrying once" >&2; \
      npm install -g @anthropic-ai/claude-code@{{.ClaudeVersion}} \
   && claude --version; } \
 || { echo "kekkai: claude native binary failed to install after retry - usually a transient npm registry/CDN failure; rerun 'kekkai up' to retry the build" >&2; \
      exit 1; }
```

Shell semantics (bash, left-associative `&&`/`||`): `((install && verify) || (notice; retry && verify)) || (message; exit 1)`.

## Guarantees

1. **Verification = `claude --version`** in the same RUN step, running as `kekkai` with the nvm bin dir on PATH (BASH_ENV-sourced nvm.sh) — no dependency on the later `/usr/local/bin/claude` symlink. Exit 0 ⇔ the native binary is present and runnable. No network access: verified to succeed under `--network none` on a complete install (~0.1s).
2. **Exactly one retry**, byte-identical to the first install command. No `npm cache clean`, no `--force`, no configurable count. The retry notice line is stderr, prefixed `kekkai:`.
3. **Double failure fails the build**: step exits 1 after writing one stderr line that MUST contain both the cause phrase `claude native binary failed to install` and the remedy phrase `rerun 'kekkai up'`. BuildKit prints a failed step's output in every progress mode (compact TTY, plain/`--verbose`, captured or not), so the line reaches the `kekkai up` user without host-side Go changes; `up` still exits with the existing `docker build failed` error.
4. **No image on failure**: docker never tags a failed build; `docker image inspect <tag>` keeps missing, so the next `up` rebuilds (no reuse of a broken image is possible for post-feature builds).
5. **Success is silent**: apart from npm's own output and one `claude --version` version line inside the step log, a passing build is byte-equivalent in user-visible behavior to pre-feature builds.

## Identity impact

- Image hash (§6.1, sha256 of rendered Dockerfile + firewall script): changes exactly once when this step lands — the normal recipe-change rebuild. `{{.ClaudeVersion}}` remains the only template variable in the step, exactly as before.
- `kekkai.config_hash` label formula: **unchanged**. The §6.2 offline fallback (which never builds) keeps matching pre- and post-feature images; it gains no checks, warnings, or failure modes.

## Out of scope

- Retroactive detection of broken pre-feature images (replaced by the forced rebuild).
- Any change to the specs/020 apt signature hint or specs/022 capture behavior — this step's message rides on BuildKit's failed-step output, not on host-side output scanning.
