# Quickstart Validation: Disable Nonessential Traffic

End-to-end per constitution IV: real binary, real docker daemon. Build once:
`go build -o /tmp/kekkai-test ./cmd/kekkai`. Expected outputs per
[contracts/nonessential-traffic-cli.md](contracts/nonessential-traffic-cli.md).

## Scenario 1 — env var present in every sandbox (US1)

```sh
mkdir scratch && cd scratch && /tmp/kekkai-test up   # in one terminal
docker exec $(docker ps -q -f name=kekkai-scratch) env | grep NONESSENTIAL
```

Expect: `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`; Claude starts normally.

## Scenario 2 — user env override wins (FR-002)

```sh
printf 'env:\n  CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: "0"\n' > .kekkai.yaml
/tmp/kekkai-test up --force
docker exec $(docker ps -q -f name=kekkai-scratch) env | grep NONESSENTIAL
```

Expect: value `0` (user wins). Remove the file afterwards.

## Scenario 3 — statsig out of the firewall allowlist (US2)

Watch `up` startup output: no `[kekkai] allowed: statsig.anthropic.com` line;
`[kekkai] allowed: api.anthropic.com` still present; both probes still pass
(`probe OK: https://example.com blocked`, `probe OK: https://api.anthropic.com
reachable`). Then from inside:

```sh
/tmp/kekkai-test shell
curl -sS --max-time 5 https://statsig.anthropic.com   # expect: blocked/refused
curl -sS -o /dev/null -w '%{http_code}\n' --max-time 10 https://api.anthropic.com  # expect: HTTP status
```

## Scenario 4 — image rebuild on upgrade (hash change)

With an image built by the previous kekkai version present: `up` prints
`building image kekkai:<newhash> ...` (firewall script changed the hash),
builds once, then subsequent `up` reuses the new image.

## Scenario 5 — docs and starter clean (US3, FR-005)

```sh
grep -ri statsig README.md SPECIFICATION.md; echo "grep exit=$?"   # expect exit=1
mkdir scratch2 && cd scratch2 && /tmp/kekkai-test init && grep -ci statsig .kekkai.yaml
```

Expect: no doc hits; starter contains zero statsig mentions and names
api.anthropic.com as the only always-allowed destination.

## Scenario 6 — regression

Existing configured project (`allow_github`, `allowed_domains`): unchanged
startup lines, github probe passes when enabled; `go build ./... && go vet
./...` clean.
