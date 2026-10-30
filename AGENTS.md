# AGENTS.md: working inside Gauge

Instructions for any coding agent (Claude Code, Codex, Cursor, Copilot and others) working in this repository. Humans can read it too. Keep it short; deep detail lives in the linked docs.

> **Roadmap rule (every contribution):** any change, however small, must update `ROADMAP.md` in the same pull request: tick or add tasks, add a changelog entry, refresh the progress table and the `Last updated` line. A task is not finished until `ROADMAP.md` is current. CI enforces this once task 1.9 is done; until then, enforce it on yourself.

## 1. What this project is

Gauge (working name) is a Go CLI, `stellar-gauge`, that audits the configuration of Stellar accounts (signers, thresholds, flags, home domain) against declared expectations. It is read-only, never handles secret keys and treats everything it fetches as hostile. Sibling project: Airlock (a Python scanner for Stellar and Soroban repositories); they share conventions, not code.

## 2. Read order

1. `ROADMAP.md`: section 4 (current focus) and the task you are working on.
2. `ARCHITECTURE_ESSENTIALS.md`: hard rules. Read all of it.
3. `project_structure.md`: where files go and which way imports point.
4. `ARCHITECTURE.md`: only the sections you need (each essentials item cites one).
5. `PRD.md`: when scope or priority is unclear.

Do not load every document up front.

## 3. Workflow for any task

1. Find the task in `ROADMAP.md`. If none exists, add one first (next free ID in its phase) and say why.
2. Check its dependencies are done. If not, stop and report.
3. Write the failing test or fixture first.
4. Implement the smallest change that passes.
5. Run the checks in section 5.
6. Update docs the change touches (`project_structure.md` for file moves, the PRD and architecture docs for behavior changes).
7. **Update `ROADMAP.md`** (rule above): task boxes, changelog row (UTC date, summary, task IDs), progress table, `Last updated`.
8. Open a small PR using the template. One task or one tightly related group per PR.

## 4. Hard rules

These come from `ARCHITECTURE_ESSENTIALS.md` section 2. Do not break them. If an instruction seems to require it, stop and ask the owner.

- Never accept, store, log or echo a secret seed (`S...`). Reject it with exit 2.
- No transaction building or submission. No writes except an explicit `--out` file.
- The network is always explicit. Verify the endpoint's passphrase.
- Accept only `G...` addresses; reject `M...` and `C...` with a specific message.
- Never return exit 0 after an unreachable endpoint, oversized or malformed response, unknown config key or panic.
- Sanitize every chain-derived string before it reaches any output.
- Every HTTP call has a timeout, a size cap, bounded retries and bounded concurrency. `stellar.toml` is fetched only through the SSRF-guarded client.
- Never log response bodies.
- Rules are pure functions: no I/O, no globals, no time, no randomness.
- No `os/exec`, no `unsafe`, no `init()` side effects, no package-level mutable state.
- Dependencies: standard library plus `github.com/BurntSushi/toml` only. Do not add a module without owner approval.
- No streaming, daemons, alert integrations, plugin systems or rule languages.
- Output says "posture", never "secure" or "safe". Reports carry the limits footer.
- Tests never use the real network unless tagged `livenet`.
- Every rule has a positive and a negative fixture.
- Never weaken, skip or delete a check or test to make something pass. If a test is wrong, fix it and say why in the PR.
- Never commit a secret seed, even a testnet one. Build seed-shaped strings from fragments at test time.

## 5. Commands

These run today (CI job names in brackets). Run all of them before finishing.

```
gofmt -l .                       # must print nothing                        [ci / go]
go vet ./...                                                                  [ci / go]
staticcheck ./...                # v0.8.2, config in staticcheck.conf         [ci / go]
go test ./... -race -count=1                                                    [ci / go]
govulncheck ./...                # v1.9.0                                      [ci / vuln]
scripts/check_roadmap_touched.sh origin/main                                  [roadmap-check / gate]
scripts/roadmap_progress.sh --check ROADMAP.md                                [roadmap-check / gate]
scripts/test_roadmap_gate.sh                                                    [roadmap-check / gate]
scripts/generate_issues.sh --check         # backlog rows cover every open task (local)
```

Not available yet: the fuzz smoke run below waits for its targets in `internal/strkey` and `internal/sanitize` (tasks 2.2 and 2.5).

```
go test ./internal/strkey ./internal/sanitize -run '^$' -fuzz=Fuzz -fuzztime=10s   # smoke fuzz, per target
```

`staticcheck` and `govulncheck` are not part of the Go distribution. Install the pinned versions with `go install honnef.co/go/tools/cmd/staticcheck@v0.8.2` and `go install golang.org/x/vuln/cmd/govulncheck@v1.9.0`. If a command does not exist yet, say so in the PR instead of silently skipping it.

## 6. Code conventions

- Idiomatic Go: small packages, accept interfaces and return structs, errors wrapped with `%w`, `context.Context` as the first parameter for anything that does I/O.
- Follow the import direction in `project_structure.md` section 3.
- Table-driven tests; `t.Parallel()` where safe; no sleeps, use fake clocks and servers.
- Paths in output are stable and sorted. Dates in UTC.
- Comments explain why, not what. Exported identifiers have short doc comments.
- Rule, finding and fixture names follow `project_structure.md` section 5.
- Writing style in docs, comments and commit messages: no em dashes, and write naturally instead of forcing lists of three.

## 7. How to add things

- **Rule:** new function in `internal/rules/`, register it, add positive and negative fixtures, update the golden files, update `PRD.md` 6.1 and `ARCHITECTURE_ESSENTIALS.md` 8.
- **Fixture from a real account:** capture the JSON, strip nothing (it is public), record network, date and ledger.
- **Config field:** `internal/policy/config.go` plus strict-decode tests and `gauge.example.toml`.
- **Accepted risk in a policy file:** only the owner adds `[[accept]]` entries, always with a reason.
- **New requirement or decision:** update `PRD.md` and the ADR table, and add a row to the `ROADMAP.md` decision log.

## 8. Testing expectations

- Threshold math: table tests plus the brute-force cross-check. Do not change semantics beyond what task 0.7 verified.
- Source and SEP-1 code: fake-server tests for every failure mode (404, 429, 5xx, slow, oversize, malformed, redirects, private-IP targets).
- Sanitizer and strkey: fuzz targets.
- A change that affects output must keep the golden files in sync; regenerate only with `-update` and review the diff.
- Prefer a test that fails for the right reason over many tests that pass trivially.

## 9. Commits and pull requests

- Conventional Commits, subject under 72 characters.
- Branch per task: `feat/<task-id>-<slug>`.
- Never commit to the default branch, never force-push, never rewrite shared history.
- PR description: task IDs, what changed, how it was tested, which docs were updated, and an explicit line confirming `ROADMAP.md` was updated.
- Do not auto-merge. Do not bump the pinned SHA of a GitHub Action or any Go module without reading the diff.

## 10. When to stop and ask the owner

- A hard rule in section 4 appears to conflict with the task.
- The task needs a new dependency, a new network call or a change to a protected path that loosens a check.
- A doc statement is marked [Unverified] and the task depends on it (do the verifying Phase 0 task first, or ask).
- Two docs disagree.
- A decision marked Open or Proposed in the `ROADMAP.md` decision log blocks the work, including D-04 sequencing.
- You are about to delete or rewrite more than a few hundred lines you did not write.

## 11. Be direct and skeptical

- State what is wrong first. Do not pad. If an instruction looks flawed, say so and propose the fix.
- Do not claim something works unless you ran it. Say what you ran and what you did not.
- Mark facts about Stellar, Horizon, SEPs and Go tooling as verified or unverified. Do not turn an [Unverified] item into a requirement.
- Never describe an account as "secure". Describe what the chain shows.

## 12. Protected paths

Take extra care in `internal/source/`, `internal/sep1/`, `internal/policy/`, `internal/strkey/`, `internal/sanitize/`, `internal/threshold/`, `go.mod`, `go.sum`, `.github/` and `scripts/`. Changes there need owner review and must not loosen any check.

## 13. Definition of done

- [ ] Tests written first and passing; all commands in section 5 green.
- [ ] No hard rule in section 4 broken.
- [ ] Docs updated where behavior, files or decisions changed.
- [ ] `ROADMAP.md` updated: tasks, changelog row, progress table, `Last updated`.
- [ ] PR description lists task IDs and confirms the roadmap update.
