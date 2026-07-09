# Tasks: Inspect egress traffic (`kekkai watch`)

**Input**: Design documents from `/specs/010-inspect-egress/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/watch-cli.md, quickstart.md

**Tests**: No unit-test tasks — constitution IV mandates end-to-end validation;
story phases run quickstart.md scenarios against a real docker sandbox
(traffic generated with `kekkai exec`, feature 009).

**Organization**: Capture plumbing (firewall rules + reader tool) is
foundational — it blocks all stories. US1 = labeled connection stream,
US2 = DNS/hostnames layered on top, US3 = error paths.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [ ] T001 Verify baseline: `go build ./... && go vet ./...` pass on clean tree

---

## Phase 2: Foundational (blocks all user stories)

- [ ] T002 In embed/init-firewall.sh add passive NFLOG rules per contracts/watch-cli.md firewall contract (research.md R1/R6): normal path — `udp dport 53 -j NFLOG --nflog-group 1` ahead of the `lo` ACCEPT, `udp sport 53 -j NFLOG --nflog-group 1` on INPUT before its ACCEPTs, `-m state --state NEW` group-1 rules immediately before the ipset ACCEPT and the bridge-subnet ACCEPT, `-m state --state NEW -j NFLOG --nflog-group 2` immediately before the REJECT; allow_all path — group-1 DNS + NEW rules before the early `exit 0`, policies stay ACCEPT, no group 2. DROP policies, ACCEPT/REJECT lines, §9.6 probes byte-for-byte untouched (FR-005, constitution II)
- [ ] T003 [P] In internal/runtime/up.go add `"tcpdump"` to `builtinAptPackages` under the subcommands tier with a `(kekkai watch)` note, mirroring `zsh` (research.md R2); `go build ./...` passes

**Checkpoint**: every new sandbox carries observe-only log rules and the reader tool

---

## Phase 3: User Story 1 - Discover which endpoints a tool needs (Priority: P1) 🎯 MVP

**Goal**: `kekkai watch` streams live `ALLOW`/`BLOCK` connection lines from the running sandbox for `$PWD`

**Independent Test**: quickstart.md scenarios 2, 3, 5 (+ scenario 1's BLOCK line, hostname annotation deferred to US2)

- [ ] T004 [US1] Create internal/runtime/watch.go: `Watch() (int, error)` — resolve running container by `LabelCwd` (error `no running sandbox for <pwd>, run 'kekkai up'` exit 1); spawn `docker exec -u root <id> tcpdump -l -n -tt -i nflog:1` and `... nflog:2` via os/exec with stdout pipes; two scanner goroutines feed one channel tagged ALLOW/BLOCK; parse tcpdump `-tt` lines into `HH:MM:SS ALLOW|BLOCK <proto> <ip>:<port>` (unparseable lines pass through raw); stderr banner `watching egress of sandbox for <pwd> (Ctrl+C to stop)`; SIGINT/SIGTERM → kill both readers + best-effort `docker exec -u root <id> pkill -x tcpdump`, exit 0; readers end on their own → stderr `sandbox stopped`, exit 1; immediate reader exit 126/127 → stderr `sandbox image predates 'kekkai watch'; run 'kekkai down' and 'kekkai up' to rebuild`, exit 1 (research.md R3-R5, exact strings in contracts/watch-cli.md)
- [ ] T005 [US1] In cmd/kekkai/main.go add `case "watch"`: any argument → error `usage: kekkai watch (no arguments)` exit 1, else `runtime.Watch()`; add the watch entry to the `usage` string per contracts/watch-cli.md help-text block; `go build ./... && go vet ./...` pass
- [ ] T006 [P] [US1] Amend SPECIFICATION.md: §3 CLI block gains `kekkai watch` line + behavior bullet (all egress labeled by verdict, DNS inline, attach-anytime, observe-only, exact strings in specs/010-inspect-egress/contracts/watch-cli.md); §5.1 required-subcommands packages gains `tcpdump` (`kekkai watch`); §9 documents the NFLOG groups and the allow_all observe-only block; update README.md command digest (constitution I, same change as code)
- [ ] T007 [US1] e2e per quickstart.md: build `/tmp/kekkai-test`, fresh `up` under pseudo-TTY (image rebuilds — expected); scenario 2 (`ALLOW tcp <ip>:443` for api.anthropic.com), scenario 1's BLOCK line (`BLOCK tcp <ip>:443` for example.com), scenario 3 (allowed_domains round-trip flips it to ALLOW), scenario 5 (watch attaches to a long-running sandbox without restart)

**Checkpoint**: verdict-labeled live stream works — MVP delivered

---

## Phase 4: User Story 2 - See hostnames, not just IP addresses (Priority: P2)

**Goal**: DNS queries/answers interleave in the stream; connection lines carry `(<hostname>)` annotations

**Independent Test**: quickstart.md scenarios 1 (full), 4

- [ ] T008 [US2] In internal/runtime/watch.go parse group-1 DNS packets from tcpdump decode: `A? <name>.` → `HH:MM:SS DNS   query <name>`; answer records → `HH:MM:SS DNS   answer <name> -> <ip>[ <ip>...]` feeding `map[ip]hostname` (last-writer-wins); annotate ALLOW/BLOCK lines as `<ip>:<port> (<hostname>)` on cache hit (data-model.md, FR-003/FR-004)
- [ ] T009 [US2] e2e per quickstart.md: scenario 1 full (`DNS   query example.com`, `DNS   answer example.com -> <ip>`, `BLOCK tcp <ip>:443 (example.com)` in order); scenario 4 (`curl http://1.1.1.1` → `BLOCK tcp 1.1.1.1:80` with no annotation)

**Checkpoint**: blocked destinations are directly actionable as config hostnames

---

## Phase 5: User Story 3 - Inspect without preparation (Priority: P3)

**Goal**: actionable errors; zero-prep attach already proven in US1

**Independent Test**: quickstart.md scenarios 8, 9

- [ ] T010 [US3] e2e per quickstart.md: scenario 8 (no sandbox → exact stderr string, exit 1), scenario 9 (`watch extra-arg` → usage error, exit 1); confirm attach required no config/flag/restart anywhere in scenarios so far (FR-009)

**Checkpoint**: all three stories independently validated

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T011 In internal/runtime/watch.go add repeat suppression: identical `(verdict, proto, ip, port)` tuple within 5s not reprinted, first occurrence always printed (data-model.md window); e2e quickstart scenario 11 (5x curl loop → one BLOCK line, log readable)
- [ ] T012 e2e per quickstart.md lifecycle/regression: scenario 6 (pty `\003` Ctrl+C → exit 0, `pgrep -x tcpdump` in sandbox empty), scenario 7 (immediate re-watch works), scenario 10 (allow_all sandbox: DNS+ALLOW only, no BLOCK), scenario 12 (traffic verdicts unaffected while watching; `down` mid-watch → `sandbox stopped`, exit 1), scenario 13 (§9.6 startup probes still pass on fresh `up`), scenario 14 (stale-image error path, skip if no pre-feature image available)
- [ ] T013 Cleanup test resources (`docker rm -f` test sandbox; remove ONLY this test dir's `kekkai-history-<hash>` volume — never bulk-remove `kekkai-history-*`) and final `go build ./... && go vet ./...`

---

## Dependencies & Execution Order

- T001 → T002/T003 (parallel: different files) → T004 → T005 → T007
- T006 [P] anytime after T001 (docs-only)
- T007 → T008 → T009 → T010 → T011 → T012 → T013
- US2 (T008) builds on US1's watch.go; US3 needs only T004/T005 behavior

## Parallel Opportunities

- T002 (firewall script) ∥ T003 (up.go) ∥ T006 (SPECIFICATION.md/README)
- e2e scenario groups within T012 share one sandbox and run back-to-back

## Implementation Strategy

MVP = Phases 2-3: passive firewall rules + reader tool + watch stream with
verdict labels (~120 net lines, dominated by watch.go). US2 adds the DNS
parsing/annotation layer inside the same file. US3 is validation only. The
image rebuild lands once (T007's first `up`) and covers all later scenarios.
Any scenario failure is fixed inside the owning story's phase before moving on;
firewall edits during iteration force cheap rebuilds on next `up` (image hash
covers the script — see project memory).
