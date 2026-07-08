# Quickstart Validation: Optional Configuration File

End-to-end per constitution IV: real binary, real docker daemon. Build once:
`go build -o /tmp/kekkai-test ./cmd/kekkai`. Use a scratch dir per scenario.
Warning line per [contracts/optional-config-cli.md](contracts/optional-config-cli.md).

## Scenario 1 — no config file: defaults + one warning (US1)

```sh
mkdir scratch1 && cd scratch1 && /tmp/kekkai-test up
```

Expect: one stderr line `warning: no .kekkai.yaml found, using defaults - run 'kekkai init' to customize`
(yellow when stderr is a TTY), then normal startup with defaults (lts image,
firewall on). Verify stderr routing: `... up 2>/dev/null` hides the warning;
`... up 2>&1 >/dev/null | grep warning` shows it plain (no ANSI codes when piped).

## Scenario 2 — empty and comments-only files: defaults, silent (US2)

```sh
: > .kekkai.yaml && /tmp/kekkai-test up          # zero-byte
printf '# hi\n' > .kekkai.yaml && /tmp/kekkai-test up   # comments-only
```

Expect: no parse error (old bug: `parse ...: EOF`), no warning line, default
startup both times.

## Scenario 3 — fresh init output runs as-is (US3)

```sh
mkdir scratch3 && cd scratch3 && /tmp/kekkai-test init
grep -cv '^\s*\(#\|$\)' .kekkai.yaml   # expect 0: no active lines
/tmp/kekkai-test up                     # defaults, no warning
```

Then uncomment `node_version` and set `24`; rerun `up`; expect a node:24 image
build (setting takes effect).

## Scenario 4 — configured projects unchanged (regression)

In an existing project with active keys: `up` behaves exactly as before, no
warning. Also verify unchanged errors: both `.yml`+`.yaml` present → "remove
one"; a malformed file (`image: [`) → parse error.

## Scenario 5 — exit status and equivalence (SC-002/FR-007)

Compare a no-config run with an all-defaults-file run: same image tag chosen,
same firewall behavior, same exit codes (TTY caveats aside).

## Scenario 6 — docs contain no stale requirement (FR-008)

```sh
grep -rn "config file is required\|needs a .kekkai\|no .kekkai.yaml found, run" README.md SPECIFICATION.md
```

Expect: no hits describing the file as required; README/SPECIFICATION describe
optional behavior and the warning.
