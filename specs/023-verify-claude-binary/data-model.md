# Data Model: Verify Claude Binary at Image Build Time

No stored data, config keys, or Go types change. The model is the state machine of one Dockerfile RUN step and its effect on image existence.

## Entities

### Claude installation (in-image)

| Attribute | Values | Meaning |
|-----------|--------|---------|
| wrapper package | present | `@anthropic-ai/claude-code` npm package (thin wrapper, ~204K) — always present after any npm exit 0 |
| native binary | present / absent | platform optionalDependency payload copied over `bin/` by the wrapper's postinstall; absent when npm silently dropped the optional dep |
| functional | yes / no | `claude --version` exits 0. `yes` ⇔ native binary present and runnable. This is the only attribute the build gates on. |

### Install-verify step (one RUN layer)

States and transitions:

```text
INSTALL#1 ──npm exit 0/≠0──▶ VERIFY#1 ──exit 0──▶ STEP OK (layer committed)
                                │ exit ≠0
                                ▼
                          INSTALL#2 (identical command, exactly once)
                                │
                                ▼
                            VERIFY#2 ──exit 0──▶ STEP OK
                                │ exit ≠0
                                ▼
                     FAIL: actionable stderr line, exit 1
                     (build fails, no image tagged)
```

Invariants:

- VERIFY is always `claude --version` — network-free, ~0.1s (research.md R1, R7).
- Exactly one retry (research.md R2); retry re-runs the *identical* install command, which re-reifies and re-downloads the missing optional dependency.
- FAIL is unreachable without both installs leaving a non-functional claude.

### Sandbox image

| State | Cause | Next `kekkai up` |
|-------|-------|------------------|
| tagged, claude functional | STEP OK | reused (inspect hit) — guaranteed-runnable claude (spec SC-001) |
| not tagged | FAIL (or any earlier build failure) | fresh build attempt (inspect miss) — the FR-002 property, free from docker's no-tag-on-failure semantics (research.md R4) |
| tagged pre-feature, claude broken | historical builds only | replaced by the one-time rebuild the Dockerfile-text hash change forces (research.md R5) |

## Relationships

- Image hash (§6.1) ← rendered Dockerfile text: changes once with this feature; never again unless the step text changes.
- `kekkai.config_hash` label (§6.2 offline-fallback key): inputs untouched — pre- and post-feature images remain interchangeable for the fallback, which never builds and thus never enters the state machine above.
