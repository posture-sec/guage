# ARCHITECTURE_ESSENTIALS: Gauge (working name)

Status: design target v0.1; implementation is a scaffold only (ROADMAP 1.12, D-09). Model declarations, policy types and the source interface exist; all audit features are pending. Full detail: `ARCHITECTURE.md` (section numbers in brackets). Scope: `PRD.md`. Work queue: `ROADMAP.md`. File locations: `project_structure.md`.

## 0. Roadmap rule (read first)

- `ROADMAP.md` must be updated in the same PR as every change: code, tests, docs, CI, config, dependency bumps, reverts.
- Update means: tick, untick or add tasks; add a changelog entry; refresh the progress table; refresh `Last updated`.
- CI check: `scripts/check_roadmap_touched.sh` (task 1.9; until it exists, reviewers enforce by hand). [16]
- If behavior changes, also update `PRD.md`, `ARCHITECTURE.md`, this file and `project_structure.md` in the same PR.

## 1. What this is

- Go CLI (`stellar-gauge`) that audits Stellar account configuration: signers, thresholds, flags, home domain.
- Compares live state with declared expectations and saved baselines. State-based, not event-based.
- Read-only. Never handles secret keys. Everything fetched is hostile. [1, 4]

## 2. Hard rules (never break)

- Never accept, store, log or echo a secret seed (`S...`). Reject with exit 2 and a rotate-the-key message. [6.6, ADR-007]
- No transaction building or submission. No writes except an explicit `--out` file. [1]
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

`policy loader + Source + sep1 -> snapshots -> rules -> accept filter -> report -> exit code` [3]

- `source`: `Network()`, `Account()`; Horizon first, RPC later. [6.4]
- `rules`: pure functions over `AccountSnapshot` plus policy. [6.7]
- `report`: text, JSON, markdown (P1); ordering severity, account, rule. [6.8]

## 5. Packages (import direction) [project_structure.md]

`model <- strkey, threshold, sanitize <- policy, source, sep1 <- rules <- report <- run <- cmd`

- `model` imports nothing internal. `rules` imports no network package. `report` imports only `model` and `sanitize`.

## 6. Data models [7]

- `Severity`: Info < Warn < High. `Role`: issuer, treasury, distribution, merchant, other.
- `Signer`: Key, Type (Ed25519, HashX, PreAuthTx), Weight, Sponsor. `Thresholds`: Low, Medium, High. `Flags`: AuthRequired, AuthRevocable, AuthImmutable, ClawbackEnabled.
- `AccountSnapshot`: ID, Network, Endpoint, Ledger, Sequence, Thresholds, Signers, Flags, HomeDomain, Sponsor, Balances.
- `Finding`: RuleID, Severity, Account, Message, Evidence, Fingerprint (`sha256(rule|account|evidence)`), Accepted.
- `NetworkInfo`: Passphrase, Ledger. `AccountReport`: Snapshot, Findings. `Report`: ToolVersion, Network, Ledger, Accounts, typed ExitCode.
- Config `gauge.toml`: `version`, `network`, `endpoints`, `[[account]]` (id, name, role, expected_signers, expected_thresholds, expected_flags, home_domain, expect_locked), `[[accept]]` (rule, account, reason, optional expires). Draft types preserve omitted expectations; strict decode and acceptance logic are not implemented.

## 7. Threshold math [6.3]

- Weights sum and must be greater than or equal to the threshold. Thresholds 0 to 255, weights 1 to 255.
- Only signers with weight above 0 participate. Pre-auth signers do not count toward standing control.
- `MinSigners`: sort weights descending, accumulate. Threshold 0 needs one weighted signer.
- Edge behavior is Unverified until task 0.7. Do not extend semantics beyond the table tests.

## 8. Rules (v0.1) [9]

`acct-missing`, `acct-network-mismatch`, `sig-no-signing-weight`, `sig-threshold-unreachable`, `sig-threshold-order`, `sig-single-key-high`, `sig-hashx`, `sig-unexpected-signer`, `sig-missing-expected`, `sig-weight-drift`, `thr-drift`, `flag-drift`, `flag-undeclared-clawback`, `id-no-home-domain`, `id-toml-unreachable`, `id-toml-account-unlisted`.

- Role profiles change severity (PRD Appendix A). Not acceptable via `[[accept]]`: `acct-missing`, `acct-network-mismatch`, `sig-threshold-unreachable`.

## 9. Exit codes [8]

- 0 pass, 1 findings at or above `fail_on`, 2 config or usage error, 3 source or internal error.

## 10. Trust and residual risk [4]

- Config is policy: protect it with CODEOWNERS and a protected branch.
- An attacker who edits the config and the scheduled workflow can make a check pass. Not solvable in code.
- Posture is not custody: key storage and holder identity are invisible on-chain.

## 11. Testing must-haves [12]

- Table plus brute-force cross-check for threshold math.
- Fake Horizon for 200, 404, 429, 5xx, slow, oversize, malformed, wrong passphrase, redirects.
- Captured real JSON fixtures for every PRD Appendix B scenario.
- SSRF, sanitizer and fuzz tests. Golden outputs. Roadmap gate script test.

## 12. Decisions (ADR one-liners) [13]

- 001 Go stdlib first. 002 state-based audit. 003 own strkey. 004 one dependency (TOML). 005 `Source` interface.
- 006 explicit network. 007 reject seeds. 008 declared expectations plus roles. 009 explicit accepts.
- 010 sanitize everything. 011 dial-time SSRF guard. 012 pure rules, no plugins. 013 roadmap gate in CI.
- 014 scope-only scaffold before features; user-approved exception, no validation gates waived.

## 13. Cut or deferred (do not build without a roadmap decision) [14.3]

- Cut: watch daemon, alert integrations, rules language, dashboard, reputation scoring, contract-account auditing, CLI framework, full Stellar SDK import.
- Deferred: RPC source, two-endpoint comparison, markdown output, snapshot and diff, trustline exposure, SEP-29 check.

## 14. Known unknowns (do not assume) [15]

- Whether Horizon lists the master key in `signers`, and its exact JSON shape.
- Threshold edge semantics (threshold 0 with no weighted signer; leftover signatures).
- SEP-1 field names, size limit, redirect rules. Horizon's long-term status. Public endpoint rate limits.
- TOML dependency's transitive footprint.

## 15. Definition of done for v0.1.0

- P0 requirements FR-01 to FR-13 met with tests.
- All Appendix B fixtures produce the expected findings; fail-closed tests green.
- Dogfooded on real operator accounts with zero false HIGH once expectations are declared.
- Attested release; `govulncheck` clean; `ROADMAP.md` current.
