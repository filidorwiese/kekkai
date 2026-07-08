# Contract: Optional Configuration File

## Missing-file warning (`kekkai up`, no config file)

Exactly one line on **stderr**, before any other output:

```
warning: no .kekkai.yaml found, using defaults - run 'kekkai init' to customize
```

Rendering:

| stderr is a terminal | NO_COLOR set | Output |
|----------------------|--------------|--------|
| yes | no | line wrapped in yellow (`ESC[33m` ... `ESC[0m`) |
| yes | yes | plain text |
| no (piped/redirected) | any | plain text |

The run then proceeds identically to a run with an all-defaults config file.
Exit status is unaffected by config absence.

## Silence table (no warning of any kind)

| Condition | Behavior |
|-----------|----------|
| `.kekkai.yaml` or `.kekkai.yml` present with active keys | today's behavior, unchanged |
| Present but empty (0 bytes / whitespace-only) | all defaults, silent |
| Present but comments-only (incl. fresh `kekkai init` output) | all defaults, silent |

## Unchanged error paths

| Condition | Behavior |
|-----------|----------|
| Both `.kekkai.yml` and `.kekkai.yaml` exist | `both .kekkai.yml and .kekkai.yaml exist, remove one` |
| Malformed YAML (real syntax error) | parse error, exit 1 |
| Legacy keys / unknown fields / validation failures | unchanged error report |
| `kekkai init` with existing config | `... already exists, not overwriting` |

## `kekkai init` starter template

- Every configuration key is commented out; the file contains only comments
  and blank lines.
- Parsed as-is it yields pure default behavior (empty YAML document).
- Commented example values for `node_version`, `version`, `args` equal the
  code defaults (copy/paste safety, SPECIFICATION §4.5).
- Header states the file is optional and settings shown are the defaults.
- Success message unchanged: `wrote .kekkai.yaml - review it, then run 'kekkai up'`.

## Removed behavior

- The hard error `no .kekkai.yaml found, run 'kekkai init'` no longer
  terminates `kekkai up`.
