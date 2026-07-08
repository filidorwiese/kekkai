# Data Model: Optional Configuration File

No persistent data. One transient resolution per `kekkai up` run:

## Configuration resolution (transient)

| On-disk state | Result | Warning |
|---------------|--------|---------|
| No `.kekkai.yaml` / `.kekkai.yml` | `config.Defaults()` | yes — one yellow stderr line |
| Empty file (0 bytes / whitespace) | all defaults (empty document) | no |
| Comments-only file (incl. fresh `kekkai init` output) | all defaults | no |
| File with active keys | today's behavior (parse, legacy keys, validation) | no |
| Both `.yml` and `.yaml` present | error: "remove one" (unchanged) | n/a |
| Malformed YAML (real syntax error) | parse error (unchanged) | n/a |

## Inputs

- **Config file presence**: `config.Discover` (unchanged semantics,
  `ErrNoConfig` sentinel).
- **Defaults**: existing code constants (`DefaultNodeVersion`,
  `DefaultClaudeVersion`, `DefaultClaudeArgs`) via new `config.Defaults()`.
- **Color decision**: stderr is a character device AND `NO_COLOR` unset.

## Invariants

- A defaults-resolved run (any of the first three rows) is behaviorally
  identical to a file containing only default values (SC-002).
- The warning appears if and only if no config file exists (SC-003).
- Exit status is never affected by config absence (FR-007).
- Validation still runs on the resolved config in all cases (defaults always
  pass it).
