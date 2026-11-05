# ARCHITECTURE_ESSENTIALS: Gauge (working name)

Status: design target v0.1; Phase 3 is complete as of 2026-11-05 and v0.1.0 now waits on Phase 4. `internal/model` (types, severity names, finding fingerprints, ordering, the fact shapes a report prints beside its findings), `internal/strkey` (SEP-23 classification and seed detection), `internal/sanitize` (control stripping, IDN flag, markdown escaping), the `internal/policy` loader (strict TOML, seed rejection, network passphrases, the Appendix A role floors), the `internal/report` renderers (text, JSON, limits footer, exit mapping), `internal/source` (Horizon client with caps, retries, strict decode, passphrase comparison), `internal/sep1` (hostname validation, dial-time SSRF guard, redirect and size limits, TOML parse), `internal/threshold` (the protocol's reachability and minimum-signer math), the `internal/run` orchestrator and the `cmd/stellar-gauge` CLI (`check`, `audit`, `version`) are implemented and tested; `internal/rules` carries all sixteen v0.1 rules plus `Facts`, so a run can exit 1 on a missing account, a network mismatch, a control configuration that cannot authorize, signer and threshold drift, flag drift, an undeclared clawback or an unverifiable identity claim, and a declared role raises the severities the catalog weights by role. Every Appendix B scenario has a fixture and a golden pair behind it since 2026-11-05 (3.6), twelve files in `testdata/horizon/`, five captured from testnet and seven built here and labelled as built, and a zero-config `check` prints the facts it read ahead of any judgement since the same day (3.7), which is the last task Phase 3 owed. Not yet: anything in Phase 4, and the P1 rules. Full detail: `ARCHITECTURE.md` (section numbers in brackets). Scope: `PRD.md`. Work queue: `ROADMAP.md`. File locations: `project_structure.md`.

## 0. Roadmap rule (read first)

- `ROADMAP.md` must be updated in the same PR as every change: code, tests, docs, CI, config, dependency bumps, reverts.
- Update means: tick, untick or add tasks; add a changelog entry; refresh the progress table; refresh `Last updated`.
- CI check: the required `roadmap-check / gate` check runs `scripts/check_roadmap_touched.sh` (task 1.9, done). [16]
- If behavior changes, also update `PRD.md`, `ARCHITECTURE.md`, this file and `project_structure.md` in the same PR.

## 1. What this is

- Go CLI (`stellar-gauge`) that audits Stellar account configuration: signers, thresholds, flags, home domain.
- Compares live state with declared expectations and saved baselines. State-based, not event-based.
- Read-only. Never handles secret keys. Everything fetched is hostile. [1, 4]

## 2. Hard rules (never break)

- Never accept, store, log or echo a secret seed (`S...`). Reject with exit 2 and a rotate-the-key message. [6.6, ADR-007]
- No transaction building or submission. No writes except an explicit `--out` file; `--out` is reserved for the P1 `snapshot` command and no shipped subcommand takes it. [1]
- `--network` (or `network` in config) is required. No default. Verify the endpoint's passphrase. [ADR-006]
- Accept only `G...` addresses. Reject `M...` and `C...` with a specific message. [6.2, 6.6]
- Never exit 0 after an unreachable endpoint, oversize or malformed response, unknown config key or panic. [8]
- Sanitize every chain-derived string (home domains, data names, `stellar.toml` content) before output. [6.8]
- All HTTP: timeouts, `io.LimitReader` size caps, bounded retries, bounded concurrency, HTTPS only (loopback in tests). [6.4]
- `stellar.toml` fetch goes through the dial-time SSRF guard only. Never use a plain `http.Get` for it. [6.5]
- Never log response bodies.
- Rules are pure functions: no I/O, no globals, no time or randomness. [6.7]
- No `os/exec`, no `unsafe`, no `init()` side effects, no package-level mutable state.
- Dependencies: standard library plus `github.com/BurntSushi/toml` only. Anything else needs owner approval. [ADR-004]
- No streaming, daemons, alert integrations, plugin systems or rule languages. [14.3]
- Posture wording only: never print "secure" or "safe". Reports carry a limits footer. [GB4]
- Tests never touch the real network unless tagged opt-in.
- No rule without a positive and a negative fixture.
- Never weaken or delete a check or test to make something pass.

## 3. Stack

- Go (current stable at start, pinned in `go.mod` and CI). Standard library: `net/http`, `encoding/json`, `crypto/sha256`, `flag`.
- TOML: `github.com/BurntSushi/toml`. Strkey: own package. [ADR-003]
- Tests: `go test -race`, `httptest`, captured JSON fixtures, golden files, native fuzzing.
- Checks: `gofmt`, `go vet`, `staticcheck`, `govulncheck`.
- Release: static binaries, build provenance attestation, pinned-by-hash CI actions.

## 4. Pipeline

`policy loader + Source + sep1 -> snapshots -> rules + facts -> accept filter -> report -> exit code` [3]

- `source`: `Network()`, `Account()`; Horizon first, RPC later. [6.4]
- `rules`: pure functions over `AccountSnapshot` plus policy, and one pure `Facts` function over the same context that returns observations rather than findings. [6.7]
- `report`: text, JSON, markdown (P1); facts printed ahead of findings, then ordering severity, account, rule. Exit reads findings only, so a fact cannot fail a run. [6.8]

## 5. Packages (import direction) [project_structure.md]

`model <- strkey, threshold, sanitize <- policy, source, sep1 <- rules <- report <- run <- cmd`

- `model` imports nothing internal. `rules` does no I/O: its direct imports are `model`, `policy`, `sep1`'s result types, `threshold`, `fmt`, `sort` and `strings`, and it never imports `source`, a `net/http` type or a client. Check that on imports (`go list -f '{{.Imports}}' ./internal/rules`), not on `-deps`, which shows `net/http` transitively because `sep1`'s types and its client share one package. `report` imports only `model` and `sanitize`.

## 6. Data models [7]

- `Severity`: Info < Warn < High. `Role`: issuer, treasury, distribution, merchant, other.
- `Signer`: Key, Type (Ed25519, HashX, PreAuthTx), Weight, Sponsor. `Thresholds`: Low, Medium, High. `Flags`: AuthRequired, AuthRevocable, AuthImmutable, ClawbackEnabled.
- `AccountSnapshot`: ID, Network, Endpoint, Ledger, Sequence, Thresholds, Signers, Flags, HomeDomain, Sponsor, Balances.
- `Finding`: RuleID, Severity, Account, Message, Evidence, Fingerprint (`sha256(rule|account|evidence)`), Accepted. Severity is outside the hash, which is what lets a role re-weight a finding without moving the key an acceptance matches on.
- `NetworkInfo`: Passphrase, Ledger. `AccountReport`: Snapshot, Facts (the run's observations: reachability per tier, what the policy declared, what the identity fetch saw; each a pointer because each has a real "no answer" state), Findings. `Report`: ToolVersion, Network, Ledger, Accounts, typed ExitCode.
- Config `gauge.toml`: `version`, `network`, `endpoints`, `[[account]]` (id, name, role, expected_signers, expected_thresholds, expected_flags, home_domain, expect_locked), `[[accept]]` (rule, account, reason, optional expires). Pointer fields preserve omitted expectations; strict decode, seed rejection, per-entry validation and the role severity floors are all implemented.

## 7. Threshold math [6.3]

Verified from the protocol implementation by task 0.7 on 2026-11-04, at stellar-core v30.0.0 commit `0e49751aa9ddf892302cca02cea7f3f5d97df9f1`, the build `testdata/horizon/root-testnet.json` names in its `core_version` field, cross-read against v29.0.0 `a9d72b0` and identical there in every cited function. `ARCHITECTURE.md` 6.3 carries every citation with a file and line range; do not restate them here and do not extend semantics past what that list decides.

- Weights sum and must be greater than or equal to the threshold. Thresholds 0 to 255, weights 1 to 255, and a stored weight above 255 is clamped to 255 from protocol 10.
- Only signers with weight above 0 participate. A zero-weight master is excluded from the candidate list by the implementation, and a zero weight on any other signer is a delete rather than a signer, so the master is the only zero Gauge can observe. Pre-auth signers do not count toward standing control.
- Ed25519, `hash(x)` and `ed25519_signed_payload` all participate in the sum, and a `T...` pre-auth entry participates only for the one transaction whose hash it holds. Gauge models all four key types: task 2.11 added the signed-payload pair, so the strings `ed25519_public_key`, `sha256_hash`, `preauth_tx` and `ed25519_signed_payload` each map to a model type and the strkey kind its key must classify as, and any other string still stops the read.
- `Reachable` sums the participating weights in descending order over at most the 20 largest and compares that with the threshold: an envelope caps at 20 signatures while an account can hold 20 signers plus a nonzero master.
- `MinSigners`: sort weights descending, accumulate. Threshold 0 needs one weighted signer, because the comparison is only reached from inside a signature match. Threshold 0 with a zero-weight master and no other signer authorizes nothing.
- Implemented 2026-11-04 by task 2.3 in `internal/threshold/threshold.go`. Pre-auth exclusion, the weight-0 rule and the 20 cap each have named table cases, and a randomized subset search checks the greedy ordering.
- Unused signatures are an error, not a spare: `txBAD_AUTH_EXTRA`.
- An unreachable or out-of-order tier is a legal stored state, because `SetOptions` validates ranges and nothing else. That is what `sig-threshold-unreachable` and `sig-threshold-order` are for.

## 8. Rules (v0.1) [9]

`acct-missing`, `acct-network-mismatch`, `sig-no-signing-weight`, `sig-threshold-unreachable`, `sig-threshold-order`, `sig-single-key-high`, `sig-hashx`, `sig-unexpected-signer`, `sig-missing-expected`, `sig-weight-drift`, `thr-drift`, `flag-drift`, `flag-undeclared-clawback`, `id-no-home-domain`, `id-toml-unreachable`, `id-toml-account-unlisted`.

- Role profiles raise severity to the Appendix A floor and never lower it; `policy.SeverityFor` is the only reader of that table and `internal/run` applies it once per account, before accepts. Not acceptable via `[[accept]]`: `acct-missing`, `acct-network-mismatch`, `sig-threshold-unreachable`.
- All sixteen are live as of 2026-11-04: task 3.1 landed the nine signer and threshold rules in `internal/rules/signers.go` and `internal/rules/thresholds.go`, and task 3.4 raised the four the catalog phrases by role at the run rather than inside a rule.
- The signer and threshold rules call `internal/threshold` and never restate the arithmetic. If you find a weight sum, a 20 cap or a zero-weight filter written inside a rule, that is the bug.
- `sig-single-key-high` emits INFO, which is Appendix A's `other` row and what `check` keeps getting because it declares no role. A role raises it, at the run. Do not move the table into the rule and do not suppress the finding.
- The three comparison rules each put a key in exactly one finding, and they fire only when `expected_signers` is declared. A declared weight of 0 is an intent to remove, not a weight to compare.
- `id-no-home-domain` has two triggers and one finding: a domain a policy declared that the chain does not show, and, for `issuer` alone, no domain at all. `expect_locked` is read for `issuer` alone, and no role row lists `sig-no-signing-weight`, so the lock declaration is not undone by adjustment.
- An `[[accept]]` expiry is a date read off UTC: the day named is the last day it holds, midnight UTC is the cutover, and a value that is not a date counts as expired rather than as open.

## 9. Exit codes [8]

- 0 pass, 1 findings at or above `fail_on`, 2 config or usage error, 3 source or internal error.
- `report.Exit` reads findings and nothing else. A fact line has no severity, so an unreachable tier printed as `(unreachable)`, a document naming another network printed as `(not this network)` and a lock declaration printed as ignored all leave the exit code where the findings put it. [6.8, 3.7]

## 10. Trust and residual risk [4]

- Config is policy: protect it with CODEOWNERS and a protected branch.
- An attacker who edits the config and the scheduled workflow can make a check pass. Not solvable in code.
- Posture is not custody: key storage and holder identity are invisible on-chain.

## 11. Testing must-haves [12]

- Table plus brute-force cross-check for threshold math.
- Fake Horizon for 200, 404, 429, 5xx, slow, oversize, malformed, wrong passphrase, redirects.
- Captured real JSON fixtures for every PRD Appendix B scenario.
- SSRF, sanitizer and fuzz tests. The guard's blocked-class, DNS-rebinding and redirect cases and the sanitizer suites are green as of task 3.5; a SEP-1 parse fuzz target still needs its own roadmap task. Golden outputs. Roadmap gate script test.
- The report's fact block is tested by what it must not do as much as by what it does: a run whose every tier is unreachable, whose document names another network and whose lock declaration is ignored still exits 0 with no findings, and a document that never parsed prints its deciding step and none of its keys. Both formats, plus the shipped `gauge.example.toml` loaded through the strict parser. [6.7, 6.8]

## 12. Decisions (ADR one-liners) [13]

- 001 Go stdlib first. 002 state-based audit. 003 own strkey. 004 one dependency (TOML). 005 `Source` interface.
- 006 explicit network. 007 reject seeds. 008 declared expectations plus roles. 009 explicit accepts.
- 010 sanitize everything. 011 dial-time SSRF guard. 012 pure rules, no plugins. 013 roadmap gate in CI.
- 014 scope-only scaffold before features; user-approved exception, no validation gates waived.

## 13. Cut or deferred (do not build without a roadmap decision) [14.3]

- Cut: watch daemon, alert integrations, rules language, dashboard, reputation scoring, contract-account auditing, CLI framework, full Stellar SDK import.
- Deferred: RPC source, two-endpoint comparison, markdown output, snapshot and diff, trustline exposure, SEP-29 check.

## 14. Known unknowns (do not assume) [15]

- Whether Horizon lists the master key in `signers`, and its exact JSON shape. **Answered** 2026-10-30 on testnet and recorded in `ARCHITECTURE.md` 6.4: it does, as an ordinary entry whose key equals `account_id`, at whatever weight `thresholds[0]` holds including 0. Mainnet is unobserved per D-16.
- Threshold edge semantics (threshold 0 with no weighted signer; leftover signatures). **Answered** 2026-11-04 from the implementation by task 0.7, together with `hash(x)` satisfaction and whether an unreachable tier can exist. Section 7 states the rules and `ARCHITECTURE.md` 6.3 cites the lines.
- The Horizon wire string for an `ed25519_signed_payload` signer, and whether `hash(x)` and pre-auth entries share the `ed25519_public_key` shape. **Answered** 2026-11-04 by task 2.11, from the code that serves it rather than from a response: the string is `ed25519_signed_payload`, and every signer entry is the same four fields whatever its type, because the struct Horizon serializes has nothing else to set. `ARCHITECTURE.md` 6.4 cites the commit the testnet endpoint names as its own build. What is still unobserved is an entry of any of the three non-ed25519 kinds in a real response, so 3.6 built those fixtures and labels them built rather than captured.
- SEP-1 field names, size limit, redirect rules. Horizon's long-term status. Public endpoint rate limits.
- TOML dependency's transitive footprint.

## 15. Definition of done for v0.1.0

- P0 requirements FR-01 to FR-13 met with tests.
- All Appendix B fixtures produce the expected findings; fail-closed tests green.
- Dogfooded on real operator accounts with zero false HIGH once expectations are declared.
- Attested release; `govulncheck` clean; `ROADMAP.md` current.
