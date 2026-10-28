# ROADMAP: Gauge (working name)

Last updated: 2026-10-28T15:10Z (repo hygiene branch `chore/1.5-repo-hygiene`)
Roadmap version: 3

This file is the single source of truth for remaining work. It is read before every task and updated after every change.

## 0. The update rule (mandatory)

**Every contribution updates this file in the same pull request.** That covers code, tests, docs, CI, configuration, dependency bumps and reverts, by humans and by agents. No exceptions.

Each PR must:

1. **Tick, untick or add tasks.** Change `[ ]` to `[x]` only when the task's "Done when" is true. Add a task (next free ID in its phase) for any work discovered along the way.
2. **Add a changelog entry** in section 9: date (UTC), PR or commit, one-line summary, task IDs touched.
3. **Refresh the progress table** in section 2 (run `scripts/roadmap_progress.sh` once it exists, task 1.9; until then count by hand).
4. **Update `Last updated`** at the top of this file.
5. **Update the decision log** (section 7) if a decision was made, and the risk register (section 8) if a risk changed.

Enforcement: `scripts/check_roadmap_touched.sh` runs in CI as a required check (task 1.9) and the PR template carries a checkbox. Until task 1.9 is done, reviewers enforce this rule by hand. A PR that only edits this file is always allowed. The rule is repeated in `PRD.md` (16), `ARCHITECTURE.md` (16), `ARCHITECTURE_ESSENTIALS.md` (0), `project_structure.md`, `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md` and the PR template.

Conventions: sizes are relative effort, not calendar time (S under a couple of hours, M about half a day, L one to two days, XL several days). They are unmeasured guesses. "Deps" lists task IDs that must be done first. Task IDs are stable: never renumber, never reuse.

## 1. What "100% ready" means

The repository is 100% ready when every task in Phases 0 to 6 is done and every gate in section 3 is green. Phase 7 is backlog and does not count toward readiness.

- **v0.1.0** = Phases 0 to 4.
- **v0.2.0** = Phases 5 and 6.

## 2. Progress

<!-- PROGRESS:START -->
| Phase | Name | Done | Total | Progress |
|---|---|---|---|---|
| 0 | Validation and decisions | 1 | 13 | 7% |
| 1 | Repository bootstrap and self-protection | 3 | 13 | 23% |
| 2 | Core | 0 | 10 | 0% |
| 3 | Rules | 0 | 7 | 0% |
| 4 | Dogfood and release v0.1.0 | 0 | 9 | 0% |
| 5 | v0.2 features | 0 | 8 | 0% |
| 6 | Release v0.2.0 and community | 0 | 5 | 0% |
| 7 (backlog) | Backlog | 0 | 4 | 0% |
| **Phases 0 to 6** | **Readiness total** | **4** | **65** | **6%** |
| Gates | G1 to G10 | 0 | 10 | 0% |
<!-- PROGRESS:END -->

## 3. Readiness gates

All must be true for 100%.

- [ ] **G1** Every P0 requirement (FR-01 to FR-13 in `PRD.md`) has passing tests.
- [ ] **G2** Every scenario in `PRD.md` Appendix B has a captured or built fixture and produces the expected findings.
- [ ] **G3** Fail-closed tests pass: unreachable endpoint, oversize body, malformed JSON, wrong passphrase, unknown config key, secret-seed input all exit non-zero.
- [ ] **G4** Dogfood: Gauge runs against real operator accounts with zero false HIGH findings once expectations are declared.
- [ ] **G5** `govulncheck`, `staticcheck` and `go vet` are clean in CI, and the repo's Scorecard score is 8 or higher.
- [ ] **G6** Releases are attested, and the verification command is documented and tested.
- [ ] **G7** `PRD.md`, `ARCHITECTURE.md`, `ARCHITECTURE_ESSENTIALS.md`, `project_structure.md`, `AGENTS.md`, `CLAUDE.md` and `README.md` agree with the code.
- [ ] **G8** Required status checks (tests and roadmap gate) are configured, and a direct push to the default branch is rejected.
- [ ] **G9** No High-rated risk in section 8 is open without a mitigation task or an accepted decision.
- [ ] **G10** This file is current: last updated within the latest merged PR.

## 4. Current focus

Task 1.13: repo hygiene for program submission, on the owner's instruction of 2026-10-28 to set the repository up following the Stellar Wave playbook. The pattern was verified against live approved repos rather than assumed: `soroauth/soroauth-go` (ruleset `protect main`, `CODEOWNERS`, `ISSUE_TEMPLATE`, `dependabot.yml`, `ci / go` style job names, an `status: unaudited` badge, labels with `area:` and `complexity:` prefixes) and `plimsoll-protocol/plimsoll-indexer` (banner, badge row, maintainer table, cross-repo table, GitBook docs).

Two things the playbook assumes that do not apply here, so they are not being built: the contract and app phases (its 4 to 9) describe a Soroban contract plus frontend, and Gauge is a Go CLI with neither. Its Phase 11 documentation site is recorded as task 5.8 rather than done now.

Feature work stays blocked on tasks 0.2 to 0.4 and the technical verification tasks. The original documents in `files/` remain untouched local briefing inputs; the root documents are authoritative for development.

## 5. Phases and tasks

### Phase 0: Validation and decisions (no code)

- [x] **0.1** First drafts of the doc set (`PRD.md`, `ARCHITECTURE.md`, `ARCHITECTURE_ESSENTIALS.md`, `project_structure.md`, `AGENTS.md`, `CLAUDE.md`, `ROADMAP.md`) including a red-team pass. Done when: files exist and carry the roadmap rule. (done 2026-10-28)
- [ ] **0.2** Decide D-01 (name), D-02 (host org) and D-03 (license). S. Done when: the decision log has an entry for each.
- [ ] **0.3** Decide D-04 sequencing: start code before or after Airlock v0.1.0 (Airlock roadmap task 5.5). S. Done when: the decision log records the choice and its reason.
- [ ] **0.4** Demand check: show the sample report and rule list to at least three real operators (the Accensa team, a merchant, an issuer or treasury holder) and ask whether they would run it on a schedule. S. Done when: answers are in the changelog; if fewer than three say yes, stop and revisit scope.
- [ ] **0.5** Search for existing self-audit tools: Stellar Laboratory, explorers, SDK samples, GitHub topics. S. Done when: findings are recorded and `PRD.md` 2.2 is updated.
- [ ] **0.6** Capture real Horizon account JSON for each Appendix B scenario (default, multisig, master weight 0, hash(x) signer, pre-auth signer, issuer with flags, clawback, missing account) from mainnet or testnet. Verify whether the master key appears in `signers` and the exact field shapes and pagination of balances. M. Done when: fixtures are in `testdata/horizon/` with sidecar notes, and `ARCHITECTURE.md` 15 item 1 is answered.
- [ ] **0.7** Verify threshold semantics with testnet experiments: threshold 0 with a weight-0 master and no other signers, leftover signatures, unreachable tiers, `hash(x)` signing. M. Deps: 0.6. Done when: results are recorded and `ARCHITECTURE.md` 6.3 marks edge behavior Verified.
- [ ] **0.8** Read SEP-1 (stellar.toml fields, size limit, redirects, content type) and SEP-29 (memo required). S. Done when: `ARCHITECTURE.md` 6.5 and `PRD.md` 6.1 name the exact fields.
- [ ] **0.9** Check Horizon's long-term status and whether Stellar RPC `getLedgerEntries` can serve account data. S. Done when: `PRD.md` risk row and ADR-005 are updated.
- [ ] **0.10** Decide ADR-003: own strkey package versus importing the Stellar Go SDK package. S. Done when: the ADR is confirmed or changed.
- [ ] **0.11** Name collision check for the chosen name and for `stellar-gauge`: GitHub, Go module index, package registries and the existing Gauge testing CLI. S. Deps: 0.2. Done when: result recorded; risk rated.
- [ ] **0.12** Check public endpoint rate limits and pick default endpoints. S. Done when: `PRD.md` 6.3 and `ARCHITECTURE.md` 6.4 are updated.
- [ ] **0.13** Review the TOML dependency (license, size, transitive dependencies, maintenance). S. Done when: ADR-004 is confirmed or an alternative is chosen.

### Phase 1: Repository bootstrap and self-protection

- [x] **1.1** Create the repository under the chosen org with the chosen license. S. Deps: 0.2. Done when: repo exists with license and default branch. (done 2026-10-28: `posture-sec/guage` exists with Apache-2.0 `LICENSE` and `main`)
- [ ] **1.2** Confirm account and org hardening: hardware-key or passkey 2FA, protected recovery email, org-wide 2FA requirement, minimal owner list. S. Done when: checked off by the owner (not verifiable by CI).
- [ ] **1.3** Configure rulesets: no force push, no deletion, required PR and status checks; add `.github/CODEOWNERS` for `internal/`, `cmd/`, `go.mod`, `go.sum`, `scripts/` and `.github/`. S. Deps: 1.1. Done when: a direct push to the default branch is rejected. CODEOWNERS and the `protect main` ruleset are in place; the rejection test has not been run, so this stays unticked.
- [x] **1.4** Scaffold the Go module: `go.mod` with the pinned Go version, `cmd/stellar-gauge`, empty `internal/` packages, staticcheck config. S. Deps: 1.1. Done when: `go build ./...` and `go test ./...` run green on the skeleton. (done 2026-10-28: `go build ./...` green, `go vet ./...` green, `gofmt -l .` empty, `go test ./... -race -count=1` green; merged in PR #1)
- [ ] **1.5** CI workflows: gofmt check, `go vet`, `staticcheck`, `go test -race`, `govulncheck`; every action pinned by hash; top-level `permissions: contents: read`. S. Deps: 1.4. Done when: CI is green on the skeleton. `.github/workflows/ci.yml` prepared in 1.13; waiting on the first green run.
- [ ] **1.6** Dependabot for `gomod` and GitHub Actions, grouped weekly. S. Deps: 1.1. Done when: config merged. Reminder: bot PRs also need a roadmap changelog line. `.github/dependabot.yml` prepared in 1.13.
- [ ] **1.7** Scorecard workflow with a token that can read branch protection. S. Deps: 1.3. Done when: Branch-Protection returns a score, not an error.
- [ ] **1.8** `SECURITY.md`, `CONTRIBUTING.md` and `.github/pull_request_template.md` (with the roadmap checkbox). S. Deps: 1.1. Done when: each carries the roadmap rule. Rewritten to the approved-repo pattern in 1.13.
- [ ] **1.9** Roadmap gate: `scripts/check_roadmap_touched.sh` (fails if the PR diff changes any file but not `ROADMAP.md`, or `Last updated` did not change), `scripts/roadmap_progress.sh` (computes and checks the progress table) and a `roadmap-check.yml` workflow made a required check. M. Deps: 1.5. Done when: a test PR without a roadmap edit fails and one with it passes. Scripts and workflow prepared in 1.13, with `scripts/test_roadmap_gate.sh` covering eight cases; the required-check part lands with 1.3.
- [ ] **1.10** `README.md` skeleton: what it is, status, quickstart placeholder, roadmap rule. S. Deps: 1.1. Done when: merged. Rewritten in 1.13 with badges, maintainer table, rule list, what-it-does-not-check section and contributor credits.
- [ ] **1.11** Copy the doc set into the repo root. S. Deps: 1.1. Done when: merged. Root copies are prepared in the scaffold; merge is pending.
- [x] **1.12** Owner-authorized scope-first scaffold, independent of unfinished Phase 0 gates (D-09). S. Done when: root brief copies, scoped package/file placeholders, model declarations, presence-aware policy types and a source interface exist; the skeleton builds, model tests pass, and the CLI exits nonzero without auditing. No feature implementation or remote protection changes are included. (done 2026-10-28: local checks in 1.4, `go run ./cmd/stellar-gauge` prints a scaffold notice and exits 3; merged in PR #1)
- [ ] **1.13** Program-submission repo hygiene, from the Stellar Wave playbook Phase 10 (owner-authorized 2026-10-28). S. Deps: 1.1. Done when: CI, roadmap-gate, Dependabot, CODEOWNERS, issue templates, README/CONTRIBUTING/SECURITY rewrites and `docs/banner.svg` are merged; the label taxonomy, repository topics and a planned issue backlog derived from this file exist on GitHub; and the pattern was verified against real approved repos rather than assumed. Remote protection settings are covered by 1.3, a release by 4.8 and a documentation site by a new 5.x task.

### Phase 2: Core

- [ ] **2.1** `internal/model`: types, severity ordering, constructors. S. Deps: 1.4. Done when: unit tests pass. Draft declarations and ordering tests are included in 1.12; finalized contracts and any required constructors remain pending.
- [ ] **2.2** `internal/strkey`: classify `G`, `M`, `C`, `S`, `T`, `X`; checksum; seed detection; reference vectors; fuzz target. M. Deps: 0.10, 2.1. Done when: vectors pass and the fuzz smoke run is clean.
- [ ] **2.3** `internal/threshold`: `Reachable` and `MinSigners` with table tests and a brute-force cross-check. M. Deps: 0.7, 2.1. Done when: all tables and the randomized cross-check pass.
- [ ] **2.4** `internal/policy` loader and CLI input validation: strict decode, `version`, explicit `network`, duplicate and unknown keys, `G`-only addresses, secret-seed rejection that never echoes the value. M. Deps: 2.2. Done when: each failure path exits 2 in tests.
- [ ] **2.5** `internal/sanitize`: control, bidi and zero-width stripping, length truncation, IDN flag, markdown escaping; fuzz target. M. Deps: 2.1. Done when: injection tests pass.
- [ ] **2.6** `internal/report` text and JSON renderers plus exit-code mapping, deterministic ordering, limits footer. M. Deps: 2.5. Done when: golden tests pass.
- [ ] **2.7** `internal/source`: `Source` interface and Horizon implementation with timeouts, size caps, strict decode, bounded retries and concurrency, passphrase check, `ErrNotFound`; fake Horizon tests for every failure mode. L. Deps: 0.6, 0.9, 0.12, 2.1. Done when: the failure-mode tests pass.
- [ ] **2.8** `internal/run` orchestration. M. Deps: 2.4, 2.6, 2.7. Done when: an end-to-end test with a fake source passes.
- [ ] **2.9** `cmd/stellar-gauge` with `check`, `audit` and `version`. S. Deps: 2.8. Done when: CLI integration tests pass.
- [ ] **2.10** Fail-closed integration tests for gate G3. M. Deps: 2.9. Done when: each case exits non-zero.

### Phase 3: Rules

- [ ] **3.1** Signer rules: `sig-no-signing-weight`, `sig-threshold-unreachable`, `sig-threshold-order`, `sig-single-key-high`, `sig-hashx`, `sig-unexpected-signer`, `sig-missing-expected`, `sig-weight-drift`, `thr-drift`. L. Deps: 2.3. Done when: each has positive and negative fixtures.
- [ ] **3.2** Flag rules: `flag-drift`, `flag-undeclared-clawback`. S. Deps: 2.1. Done when: fixtures pass.
- [ ] **3.3** Account and network rules: `acct-missing`, `acct-network-mismatch`. S. Deps: 2.7. Done when: fixtures pass.
- [ ] **3.4** Role profiles and accepted risks, including non-acceptable rules and expiry handling in UTC. M. Deps: 2.4, 3.1. Done when: the same account yields different severities per role and accepts never hide a non-acceptable rule.
- [ ] **3.5** `internal/sep1`: guarded fetcher (hostname validation, dial-time IP guard, redirect limits, size cap, timeout) and identity rules `id-no-home-domain`, `id-toml-unreachable`, `id-toml-account-unlisted`. L. Deps: 0.8, 0.13. Done when: SSRF and DNS-rebinding tests pass.
- [ ] **3.6** Fixtures and golden outputs for every Appendix B scenario. M. Deps: 3.1 to 3.5. Done when: gate G2 holds.
- [ ] **3.7** Facts report and zero-config defaults; `gauge.example.toml`. M. Deps: 2.6, 3.4. Done when: a zero-config `check` prints facts plus universal rules.

### Phase 4: Dogfood and release v0.1.0

- [ ] **4.1** Dogfood on real operator accounts (Accensa and any willing merchant or issuer); declare expectations; review every finding for false positives. M. Deps: 3.7, 0.4. Done when: gate G4 holds.
- [ ] **4.2** Usage docs: install, `check`, `audit`, config reference, rule reference. M. Deps: 3.7. Done when: every rule has an ID, severity and rationale in the docs.
- [ ] **4.3** `README.md` complete with an explicit "what this does not check" section and the limits wording used in report footers. S. Deps: 4.2. Done when: merged.
- [ ] **4.4** CI matrix for Linux, macOS and Windows; cross-platform build check. S. Deps: 1.5. Done when: green on all three.
- [ ] **4.5** Release workflow with build provenance attestation; documented and tested verification command. M. Deps: 1.5. Done when: a test release verifies. Gate G6.
- [ ] **4.6** Reach a Scorecard score of 8 or higher and keep `govulncheck` clean. M. Deps: 1.7. Done when: scores recorded. Gate G5.
- [ ] **4.7** Wire Airlock into this repo's CI as a scanner (after Airlock v0.1.0). S. Deps: Airlock task 5.5. Done when: the scan runs and passes.
- [ ] **4.8** Tag `v0.1.0`; publish checksums and the verification steps. S. Deps: 4.1 to 4.6. Done when: tag exists and gates G1 to G10 are reviewed for v0.1.
- [ ] **4.9** Announce with measured results (accounts audited, issues found), not intentions. S. Deps: 4.8. Done when: published.

### Phase 5: v0.2 features

- [ ] **5.1** Multi-endpoint comparison with `src-disagreement`. M. Deps: 4.8.
- [ ] **5.2** `snapshot` and `diff` with baseline files and the review banner; documented guidance to store baselines outside the audited repo. M. Deps: 4.8.
- [ ] **5.3** Markdown output with safe escaping and fenced evidence. M. Deps: 2.5.
- [ ] **5.4** Scheduled GitHub Action template that opens an issue on findings, with minimal permissions and SHA pinning. M. Deps: 5.2, 5.3.
- [ ] **5.5** Trustline exposure facts (issuer flags for each held asset). M. Deps: 0.6.
- [ ] **5.6** SEP-29 memo-required check for merchant accounts. S. Deps: 0.8.
- [ ] **5.7** Performance and rate-limit tests with many accounts. S. Deps: 2.7.
- [ ] **5.8** Documentation site separate from the README (GitBook or equivalent): introduction with cited figures, protocol mechanics and the economic model with worked numbers, contract and rule reference, end-user guides per persona, developer guide, contributing guide. M. Deps: 4.2, 4.3. Done when: the site is linked from `README.md` and every page reflects the shipped behavior.

### Phase 6: Release v0.2.0 and community

- [ ] **6.1** Update every doc for v0.2 behavior. M. Deps: Phase 5. Gate G7.
- [ ] **6.2** Tag `v0.2.0` with attestation. S. Deps: 6.1.
- [ ] **6.3** Review false-positive reports from external users and adjust rules. M. Deps: 6.2.
- [ ] **6.4** Decide D-07: pursue funding or program applications, with adoption evidence. M. Deps: 6.2.
- [ ] **6.5** Outreach to Stellar issuers, anchors and treasury operators. S. Deps: 6.2.

### Phase 7: Backlog (not counted toward readiness)

- [ ] **7.1** Stellar RPC source implementation behind the `Source` interface.
- [ ] **7.2** Claimable balances, liquidity pool shares and sponsorship analysis.
- [ ] **7.3** Alert integrations (probably never; JSON and CI cover it).
- [ ] **7.4** Cross-check issued assets against `stellar.toml` currencies.

## 6. Dependency map (critical path)

`0.2, 0.4 -> 1.1 -> 1.4 -> 2.1 -> 2.2, 2.3, 2.5 -> 2.7 -> 2.8 -> 2.9 -> 3.1 to 3.5 -> 3.7 -> 4.1 -> 4.8`

Parallelizable early: 0.5 to 0.13 (research, no code), 1.2, 1.6, 1.8, 1.10. Land the roadmap gate (1.9) before most Phase 2 work so every later PR is checked.

## 7. Decision log

| ID | Decision | Status | Notes |
|---|---|---|---|
| D-01 | Project and repo name | Scaffold identity confirmed 2026-10-28 | Existing repo `guage`, working product name Gauge, binary `stellar-gauge`; final naming and collision check remain in 0.2 and 0.11 |
| D-02 | Host organization | Decided 2026-10-28: `posture-sec` | Owner approved the existing origin; module `github.com/posture-sec/guage` |
| D-03 | License | Decided 2026-10-28: Apache-2.0 | Owner approved retaining the existing LICENSE |
| D-04 | Sequencing against Airlock | Feature sequencing remains proposed | Scaffold-only exception in D-09; no feature work before the unresolved Phase 0 gates |
| D-05 | Data source | Proposed: Horizon first behind an interface | RPC in backlog; task 0.9 |
| D-06 | TOML parser | Proposed: `github.com/BurntSushi/toml` | Task 0.13 |
| D-07 | Funding or program applications | Open | After v0.2.0 and adoption evidence |
| D-08 | Roadmap gate has no exemptions, including dependency bumps | Decided 2026-10-28 | Matches the owner's instruction for Airlock; assumed to apply here. Dependabot is grouped weekly |
| D-09 | Scope-first scaffold before feature development | Decided 2026-10-28 | Owner approved task 1.12 as a narrow exception to the no-code gate: root documents, placeholders and draft data models only. Demand, protocol verification and feature gates remain open; no deployment or repository-protection changes |
| D-10 | Scaffold toolchain | Decided 2026-10-28 | Go 1.27.3 returned by the official stable-download metadata and verified locally with `go version`; no third-party runtime module added |
| D-11 | Adopt the Stellar Wave playbook's Phase 10 hygiene for this repo | Decided 2026-10-28 | Owner instruction. Pattern verified against `soroauth/soroauth-go` and `plimsoll-protocol/plimsoll-indexer`, not recalled. Its contract and app phases (4 to 9) do not apply to a Go CLI with no Soroban contract, so they are skipped rather than adapted |
| D-12 | No release tag until an auditing build exists | Decided 2026-10-28 | The playbook's Phase 10 item 6 asks for a `v0.1.0` tag. Tagging the scaffold would advertise a release that audits nothing, so the tag stays with task 4.8 |

## 8. Risk register

| ID | Risk | Rating | Mitigation or task |
|---|---|---|---|
| R-1 | Nobody wants it | Medium | 0.4 before feature implementation; owner-authorized scaffold-only exception D-09 |
| R-2 | Maintainer overload across three projects | High | D-04, 0.3; Phase 7 closed |
| R-3 | Threshold or signer semantics modeled wrong | High | 0.6, 0.7, 2.3 |
| R-4 | Horizon deprecation or reshaping | Medium | 0.9, ADR-005 |
| R-5 | "No findings" read as "safe" | High | 4.3, report footer, wording rule |
| R-6 | Expectations edited to match an attacker's change | Medium | 1.3, 5.2, documented residual risk |
| R-7 | `stellar.toml` fetch abuse | Medium | 3.5 |
| R-8 | Name collision | Medium | 0.11 |
| R-9 | Secret seed pasted into the CLI | Medium | 2.4 |

## 9. Changelog

Newest first. One row per merged change.

| Date (UTC) | Change | Tasks |
|---|---|---|
| 2026-10-28 | Repo hygiene for program submission: CI and roadmap-gate workflows, gate and progress scripts with an eight-case self-test, Dependabot, CODEOWNERS, issue forms, README/CONTRIBUTING/SECURITY rewritten to the verified approved-repo pattern, banner, plus a `protect main` ruleset, label taxonomy, topics and a roadmap-derived issue backlog | 1.13, 1.3, 1.5, 1.6, 1.8, 1.9, 1.10, D-11, D-12, 5.8 |
| 2026-10-28 | Owner-authorized scaffold: root brief copies, Go module, package and file placeholders, model and policy declarations, source interface, non-operational CLI, README/CONTRIBUTING/SECURITY/PR-template drafts, structure and decision documentation | 1.1, 1.4, 1.8, 1.10, 1.11, 1.12, D-01 to D-04, D-09, D-10 |
| 2026-10-28 | First drafts of the full doc set (PRD, ARCHITECTURE, ESSENTIALS, project_structure, AGENTS, CLAUDE, ROADMAP) with a red-team pass | 0.1 |
