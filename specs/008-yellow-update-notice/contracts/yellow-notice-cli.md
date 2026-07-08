# Contract: Yellow Update Notice

## Rendering (`kekkai up`, newer release available)

The notice line (text per the frozen feature-005 contract):

```
A new version of kekkai is available (<installed> -> <latest>), run 'kekkai self-update' to upgrade
```

| stdout is a terminal | NO_COLOR set | Output |
|----------------------|--------------|--------|
| yes | no | line wrapped in yellow (`ESC[33m` ... `ESC[0m`) |
| yes | yes | plain text |
| no (piped/redirected) | any | plain text |

Identical wrapping and gating as the missing-config warning (feature 006),
except the checked stream is stdout (where the notice lives).

## Frozen behavior

- Notice text (color stripped), stream (stdout), position (immediately
  before interactive handoff), and every silence condition of the
  feature-005 contract: byte-identical.
- Exit status never affected.
- Missing-config warning rendering (stderr) unchanged.
