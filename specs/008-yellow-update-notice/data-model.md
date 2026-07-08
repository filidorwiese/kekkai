# Data Model: Yellow Update Notice

No data, no state. One rendering rule shared by kekkai's two advisory lines:

| Advisory | Stream | Colored when | Text |
|----------|--------|--------------|------|
| Missing-config warning (006) | stderr | stderr is a terminal AND `NO_COLOR` unset | unchanged |
| Update notice (005, this feature) | stdout | stdout is a terminal AND `NO_COLOR` unset | unchanged |

Yellow = `ESC[33m` ... `ESC[0m`, applied by one shared helper so the two
lines cannot diverge (FR-004). Silence conditions of both advisories are
orthogonal to rendering and unchanged.
