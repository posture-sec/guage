# ROADMAP: Gauge (working name)

Last updated: 2026-10-30T10:57Z (Phase 0 verification pass and the decisions it settled)
Roadmap version: 3

This file is the single source of truth for remaining work. It is read before every task and updated after every change.

## 0. The update rule (mandatory)

**Every contribution updates this file in the same pull request.** That covers code, tests, docs, CI, configuration, dependency bumps and reverts, by humans and by agents. No exceptions.

Each PR must:

1. **Tick, untick or add tasks.** Change `[ ]` to `[x]` only when the task's "Done when" is true. Add a task (next free ID in its phase) for any work discovered along the way.
2. **Add a changelog entry** in section 9: date (UTC), PR or commit, one-line summary, task IDs touched.
3. **Refresh the progress table** in section 2 with `scripts/roadmap_progress.sh`; `--check` fails when the committed numbers are stale.
4. **Update `Last updated`** at the top of this file.
5. **Update the decision log** (section 7) if a decision was made, and the risk register (section 8) if a risk changed.

Enforcement (task 1.9, done 2026-10-28): `roadmap-check / gate` runs `scripts/check_roadmap_touched.sh`, `scripts/roadmap_progress.sh --check` and `scripts/test_roadmap_gate.sh`, and the context is listed in the `protect main` required checks. The PR template carries the matching checkbox. A PR that only edits this file is always allowed. The rule is repeated in `PRD.md` (16), `ARCHITECTURE.md` (16), `ARCHITECTURE_ESSENTIALS.md` (0), `project_structure.md`, `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md` and the PR template.

Conventions: sizes are relative effort, not calendar time (S under a couple of hours, M about half a day, L one to two days, XL several days). They are unmeasured guesses. "Deps" lists task IDs that must be done first. Task IDs are stable: never renumber, never reuse.

## 1. What "100% ready" means

The repository is 100% ready when every task in Phases 0 to 6 is done and every gate in section 3 is green. Phase 7 is backlog and does not count toward readiness.

- **v0.1.0** = Phases 0 to 4.
- **v0.2.0** = Phases 5 and 6.

## 2. Progress

<!-- PROGRESS:START -->
| Phase | Name | Done | Total | Progress |
|---|---|---|---|---|
| 0 | Validation and decisions | 10 | 13 | 76% |
| 1 | Repository bootstrap and self-protection | 13 | 15 | 86% |
| 2 | Core | 0 | 10 | 0% |
| 3 | Rules | 0 | 7 | 0% |
| 4 | Dogfood and release v0.1.0 | 0 | 9 | 0% |
| 5 | v0.2 features | 0 | 8 | 0% |
| 6 | Release v0.2.0 and community | 0 | 5 | 0% |
| 7 (backlog) | Backlog | 0 | 4 | 0% |
| **Phases 0 to 6** | **Readiness total** | **23** | **67** | **34%** |
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

Feature work is no longer blocked by sequencing: D-04 is Decided (proceed now, share conventions with Airlock) and 0.2 and 0.3 are closed. What still blocks specific tasks is named in each task: 0.4 needs three real operators before G4 dogfooding can be claimed, 0.6 needs its fixtures committed before the `source` parser is written against real shapes, and 0.7 cannot be closed by observation because the states it wants to see require a transaction submission that this project never performs. The original documents in `files/` remain untouched local briefing inputs; the root documents are authoritative for development.

Phase 0 is 10 of 13 done as of 2026-10-30, on the owner's instruction to move the repository out of scaffold-only status. The decision tasks (0.2, 0.3), the landscape search (0.5), the SEP reading (0.8), the Horizon and RPC status check (0.9), the strkey decision (0.10), the name collision check (0.11), the endpoint and rate-limit check (0.12) and the TOML dependency review (0.13) each record what was run or read, with the numbers, in their own task line and in `PRD.md` and `ARCHITECTURE.md`. Two results change the design: SEP-23 decodes only if the implementation rejects strkeys that Go's unpadded base32 decoder accepts silently (`ARCHITECTURE.md` 6.2), and a live `stellar.toml` can answer HTTP 200 with an HTML body, so the identity check keys on a successful strict TOML parse rather than on a content type (`ARCHITECTURE.md` 6.5). Verification ran against testnet only (D-16), so mainnet figures in these documents are quoted from official documentation and labelled as not observed.

Phase 1 is 13 of 15 done as of 2026-10-30: PR #2 merged into `main` as `ae6a05d`, so the repository's default branch now carries CI, the roadmap gate, Dependabot, CODEOWNERS, issue forms and the rewritten documents, and the first push-event CI run on `main` passed (`ci / go` 57s, `ci / vuln` 38s). The direct-push test that task 1.3 asked for was then run against `main` itself and refused, so 1.3 and 1.13 are closed. The issue backlog stands at 73 issues, #5 to #77, one per open task in Phases 0 to 6 plus the ten gates, regenerated with `scripts/generate_issues.sh --apply`. Task 1.14 is closed the same way: `scripts/generate_issues.sh --check` is a step in the gate workflow, and a throwaway PR that added a task with no backlog row failed on that step in 7s. Phase 1 is now 13 of 15: Dependabot (1.6), the OSSF Scorecard (1.7), the three contributor documents (1.8), the README (1.10) and the root doc set (1.11) are ticked against `main`, with the Scorecard's Branch-Protection check returning a score of 4 rather than an error and the three remaining warnings accepted under D-14 because the repository has a single maintainer. What is left in Phase 1 is the owner's hardening check (1.2), which CI cannot observe, and the continuous wave-packet upkeep task (1.15).

Wave preparation (2026-10-30, D-13): the guide's submission steps were verified against the live Drips Wave docs rather than taken from it, and the result is that no Stellar Wave is open to apply to. The repository is not applied while its CLI audits nothing, because a rejected repository cannot be re-applied. `docs/WAVE_SUBMISSION.md` holds the verified facts, the gate list that must clear before the Apply button, the maintainer runbook for a cycle, and the draft submission answers; task 1.15 keeps it current.

## 5. Phases and tasks

### Phase 0: Validation and decisions (no code)

- [x] **0.1** First drafts of the doc set (`PRD.md`, `ARCHITECTURE.md`, `ARCHITECTURE_ESSENTIALS.md`, `project_structure.md`, `AGENTS.md`, `CLAUDE.md`, `ROADMAP.md`) including a red-team pass. Done when: files exist and carry the roadmap rule. (done 2026-10-28)
- [x] **0.2** Decide D-01 (name), D-02 (host org) and D-03 (license). S. Done when: the decision log has an entry for each. (done 2026-10-30: the log carries a Decided entry for all three: D-02 `posture-sec`, D-03 Apache-2.0, D-01 working name Gauge with binary `stellar-gauge` in module `github.com/posture-sec/guage`. The collision evidence from 0.11 sits under D-15, which stays Proposed because renaming the repository slug is the owner's call.)
- [x] **0.3** Decide D-04 sequencing: start code before or after Airlock v0.1.0 (Airlock roadmap task 5.5). S. Done when: the decision log records the choice and its reason. (done 2026-10-30: D-04 recorded as Decided: proceed now and share conventions with Airlock. `gh api orgs/posture-sec/repos` lists exactly one repository, `guage`, so there is no public Airlock release to wait for, and the 0.5 landscape search found nothing that duplicates this job. This removes the AGENTS.md section 10 blocker on feature work.)
- [ ] **0.4** Demand check: show the sample report and rule list to at least three real operators (the Accensa team, a merchant, an issuer or treasury holder) and ask whether they would run it on a schedule. S. Done when: answers are in the changelog; if fewer than three say yes, stop and revisit scope. Still open: this needs three named humans, and no amount of code or searching answers it.
- [x] **0.5** Search for existing self-audit tools: Stellar Laboratory, explorers, SDK samples, GitHub topics. S. Done when: findings are recorded and `PRD.md` 2.2 is updated. (done 2026-10-30: `PRD.md` 2.2 rewritten with the repos actually returned by `gh search repos` on that date, each quoted from its own description. The nearest is `signerreach/signerreach`, self-described as an "Offline checker for changes to Stellar classic-account signers and thresholds"; the official tools found (`stellar/account-viewer-v2`, `stellar/stellar-account-prometheus-exporter`) display or export state rather than compare it with a declared expectation.)
- [ ] **0.6** Capture real Horizon account JSON for each Appendix B scenario (default, multisig, master weight 0, hash(x) signer, pre-auth signer, issuer with flags, clawback, missing account) from mainnet or testnet. Verify whether the master key appears in `signers` and the exact field shapes and pagination of balances. M. Done when: fixtures are in `testdata/horizon/` with sidecar notes, and `ARCHITECTURE.md` 15 item 1 is answered. In progress 2026-10-30: the testnet captures exist and answer the parsing question: the master key **is** listed inside `signers` as `{"weight":1,"key":"<account_id>","type":"ed25519_public_key"}`, and `/accounts/{id}/balances` is not a route (404 `Resource Missing`), so balances come only from the array embedded in the account. The scenarios that need a transaction to build (master weight 0, `hash(x)`, pre-auth) are not covered yet; committing the fixtures is the remaining step.
- [ ] **0.7** Verify threshold semantics with testnet experiments: threshold 0 with a weight-0 master and no other signers, leftover signatures, unreachable tiers, `hash(x)` signing. M. Deps: 0.6. Done when: results are recorded and `ARCHITECTURE.md` 6.3 marks edge behavior Verified. Not started; see the constraint in the phase note below: building the state that exercises these needs a transaction submission, which this project does not do.
- [x] **0.8** Read SEP-1 (stellar.toml fields, size limit, redirects, content type) and SEP-29 (memo required). S. Done when: `ARCHITECTURE.md` 6.5 and `PRD.md` 6.1 name the exact fields. (done 2026-10-30 against SEP-1 v2.7.0 and SEP-29 as fetched from `stellar/stellar-protocol` on that date. SEP-1 fixes the path `https://DOMAIN/.well-known/stellar.toml`, a 100 KB maximum file size, a mandatory `Access-Control-Allow-Origin: *`, and `text/plain` as recommended rather than required; the identity fields are `VERSION`, `NETWORK_PASSPHRASE`, `SIGNING_KEY`, `HORIZON_URL` and `ACCOUNTS` (a list of `G...` strings), and there is no `ACCOUNT_ID`. The spec sets no redirect or status-code rules, so the 3-redirect cap in `ARCHITECTURE.md` 6.5 is Gauge's own bound, not a SEP rule. SEP-29 is the `config.memo_required` account data entry whose value is the base64 of `1`, applies only to non-muxed destinations, is enforced client-side, and must block submission rather than warn.)
- [x] **0.9** Check Horizon's long-term status and whether Stellar RPC `getLedgerEntries` can serve account data. S. Done when: `PRD.md` risk row and ADR-005 are updated. (done 2026-10-30: Horizon is not marked deprecated in the developer docs, which still document it as the API surface, while SDF material steers new builders toward Stellar RPC. `getLedgerEntries` was called live against `https://soroban-testnet.stellar.org` with the 40-byte classic `LedgerKey` (ACCOUNT discriminant, `AccountID` key-type union, 32-byte key) and returned the account entry: `latestLedger` 5466028, an 84-byte `xdr` starting `000000000000000055c6953f`, no `contentType` field, and `lastModifiedLedgerSeq` 5465023, the ledger that funded that account. So an RPC source is feasible later without touching the `Source` interface, which is why ADR-005 stays Horizon-first rather than Horizon-only.)
- [x] **0.10** Decide ADR-003: own strkey package versus importing the Stellar Go SDK package. S. Done when: the ADR is confirmed or changed. (done 2026-10-30: ADR-003 confirmed, own package. Evidence is a stdlib-only encoder and decoder written against the SEP-23 specification and run against that document's own test cases: all 10 valid strkeys decode to the stated type and payload length, and all 20 invalid strkeys are rejected, each for the reason SEP-23 names. Three rules are easy to miss and were missed once each during the exercise, which is the argument for the round-trip check described in `ARCHITECTURE.md` 6.2.)
- [x] **0.11** Name collision check for the chosen name and for `stellar-gauge`: GitHub, Go module index, package registries and the existing Gauge testing CLI. S. Deps: 0.2. Done when: result recorded; risk rated. (done 2026-10-30: recorded as D-15 with the numbers behind it. The bare word is contested: `getgauge/gauge` has 3,195 stars and pushed 2026-10-29, `npm/gauge` 320, and the exact slug spelling `guage` is in use by `evoziosk/guage` (Python, 4 stars, pushed 2026-10-13), with the GitHub account name `guage` taken. The binary name is quiet: the only `StellarGauge` repository is `zvere1111/StellarGauge`, TypeScript, 1 star, last push 2025-12-17, and `github.com/posture-sec/guage` resolves on the module proxy with no tagged versions while no Go module publishes a `stellar-gauge` command. Rated: medium-high for discoverability of the repository, low for the binary name, and it is the owner's decision, so D-15 stays Proposed.)
- [x] **0.12** Check public endpoint rate limits and pick default endpoints. S. Done when: `PRD.md` 6.3 and `ARCHITECTURE.md` 6.4 are updated. (done 2026-10-30: the Horizon rate-limiting page states per-IP limiting through `PER_HOUR_RATE_LIMIT`, default 3600 requests per hour, disabled at 0, returning `429 Too Many Requests` when exceeded. Observed against `https://horizon-testnet.stellar.org`: responses are `application/hal+json; charset=utf-8`, errors are `application/problem+json`, a successful account fetch carries a `Latest-Ledger` header, `HEAD` is refused with 405, no rate-limit headers are returned at all, and `limit` above 200 is rejected with `extras.reason` "invalid limit: value provided that is over limit max of 200". Defaults recorded in `PRD.md` 6.3: mainnet `https://horizon.stellar.org`, testnet `https://horizon-testnet.stellar.org`. Mainnet was **not** probed, per D-16, so only the testnet figures are observations.)
- [x] **0.13** Review the TOML dependency (license, size, transitive dependencies, maintenance). S. Done when: ADR-004 is confirmed or an alternative is chosen. (done 2026-10-30: D-06 and ADR-004 confirmed for `github.com/BurntSushi/toml` v1.6.0, read from the repository on that date: `license.spdx_id` MIT, latest release tag `v1.6.0`, `pushed_at` 2026-08-18T20:04:59Z, 5,028 stars, not archived, and its own `go.mod` declares `module github.com/BurntSushi/toml` / `go 1.21` with **no `require` directives**, so the dependency brings nothing with it. The hard rule "standard library plus one reviewed TOML parser" holds literally.)

Constraint on the two remaining experimental tasks: Gauge never builds or submits a transaction, so the account states that only a transaction can create (master weight 0, `hash(x)` and pre-auth signers, a transaction with leftover signatures) cannot be produced for observation. Those stay hand-built fixtures plus documented Stellar behaviour marked unverified rather than quietly presented as tested.

### Phase 1: Repository bootstrap and self-protection

- [x] **1.1** Create the repository under the chosen org with the chosen license. S. Deps: 0.2. Done when: repo exists with license and default branch. (done 2026-10-28: `posture-sec/guage` exists with Apache-2.0 `LICENSE` and `main`)
- [ ] **1.2** Confirm account and org hardening: hardware-key or passkey 2FA, protected recovery email, org-wide 2FA requirement, minimal owner list. S. Done when: checked off by the owner (not verifiable by CI).
- [x] **1.3** Configure rulesets: no force push, no deletion, required PR and status checks; add `.github/CODEOWNERS` for `internal/`, `cmd/`, `go.mod`, `go.sum`, `scripts/` and `.github/`. S. Deps: 1.1. Done when: a direct push to the default branch is rejected. (done 2026-10-30: `.github/CODEOWNERS` and the `protect main` ruleset (id 26055352) are on `main` after PR #2, and the literal test ran there: `git push origin probe:main` with one empty commit was refused with "Changes must be made through a pull request" plus "3 of 3 required status checks are expected". `main` stayed at `ae6a05d` and the probe commit was dropped locally. An identical-rules probe on a throwaway branch gave the same refusal earlier the same day.)
- [x] **1.4** Scaffold the Go module: `go.mod` with the pinned Go version, `cmd/stellar-gauge`, empty `internal/` packages, staticcheck config. S. Deps: 1.1. Done when: `go build ./...` and `go test ./...` run green on the skeleton. (done 2026-10-28: `go build ./...` green, `go vet ./...` green, `gofmt -l .` empty, `go test ./... -race -count=1` green; merged in PR #1)
- [x] **1.5** CI workflows: gofmt check, `go vet`, `staticcheck`, `go test -race`, `govulncheck`; every action pinned by hash; top-level `permissions: contents: read`. S. Deps: 1.4. Done when: CI is green on the skeleton. (done 2026-10-28: `.github/workflows/ci.yml` on PR #2, jobs `ci / go` pass 1m1s and `ci / vuln` pass 33s against head `8846690`; actions pinned by full SHA, `permissions: contents: read` at workflow level)
- [x] **1.6** Dependabot for `gomod` and GitHub Actions, grouped weekly. S. Deps: 1.1. Done when: config merged. Reminder: bot PRs also need a roadmap changelog line. `.github/dependabot.yml` prepared in 1.13. (done 2026-10-30: `.github/dependabot.yml` is on `main` after PR #2, covering `gomod` and `github-actions` grouped weekly. Two `Dependabot Updates` runs completed success at 08:01:07Z on the merge commit and opened no pull requests, which is correct for now: every action is pinned by SHA at current versions, and `go.sum` does not exist until the approved dependency lands.)
- [x] **1.7** Scorecard workflow with a token that can read branch protection. S. Deps: 1.3. Done when: Branch-Protection returns a score, not an error. Prepared 2026-10-30: `.github/workflows/scorecard.yml` runs `ossf/scorecard-action` v2.5.0 pinned by SHA on pushes to `main` and weekly, with `security-events: write` and `id-token: write`, publishing SARIF to code scanning and the results to api.scorecard.dev for the README badge. The ruleset is readable by the default workflow token, which is why no admin token is used. Done 2026-10-30: the first run on `main` (created 08:22:21Z, success) published results read from `https://api.scorecard.dev/projects/github.com/posture-sec/guage`: **Branch-Protection scored 4, not an error**, with `allow deletion` disabled, `force pushes` disabled, protection applying to administrators, stale-review dismissal, up-to-date branches, a required status check and PRs required, and three warnings (no required approvers, no CODEOWNERS review requirement, no last-push approval). Overall score 6.2 from Scorecard v5.6.0. `Pinned-Dependencies` 10 (8 of 8 GitHub-owned and 1 of 1 third-party actions pinned, 2 of 2 `goCommand` pins), `Token-Permissions` 10, `Dependency-Update-Tool` 10, `Security-Policy` 10, `Dangerous-Workflow` 10, `Vulnerabilities` 10, `Binary-Artifacts` 10, `CI-Tests` 7, and 0 or not-found for `Maintained`, `Code-Review`, `Contributors`, `Fuzzing`, `SAST`, `CII-Best-Practices`, `Packaging`, `Signed-Releases`.
- [x] **1.8** `SECURITY.md`, `CONTRIBUTING.md` and `.github/pull_request_template.md` (with the roadmap checkbox). S. Deps: 1.1. Done when: each carries the roadmap rule. Rewritten to the approved-repo pattern in 1.13. (done 2026-10-30: `SECURITY.md`, `CONTRIBUTING.md` and `.github/pull_request_template.md` are on `main`, each carrying the roadmap rule; `git show origin/main:<file> | grep -c ROADMAP.md` reports 1, 5 and 2 respectively.)
- [x] **1.9** Roadmap gate: `scripts/check_roadmap_touched.sh` (fails if the PR diff changes any file but not `ROADMAP.md`, or `Last updated` did not change), `scripts/roadmap_progress.sh` (computes and checks the progress table) and a `roadmap-check.yml` workflow made a required check. M. Deps: 1.5. Done when: a test PR without a roadmap edit fails and one with it passes. (done 2026-10-28: PR #4 changed one `testdata/` file and no roadmap, `roadmap-check / gate` failed in 5s with `check_roadmap_touched: ROADMAP.md must be updated in the same pull request`; PR #2 edits the roadmap and the same check passes in 6s. Both ran against base `chore/1.5-repo-hygiene` because `main` had no workflow files until that PR merges. The context is already listed in the `protect main` required checks, so it becomes blocking for `main` with 1.3.)
- [x] **1.10** `README.md` skeleton: what it is, status, quickstart placeholder, roadmap rule. S. Deps: 1.1. Done when: merged. Rewritten in 1.13 with badges, maintainer table, rule list, what-it-does-not-check section and contributor credits. (done 2026-10-30: `README.md` on `main` with the banner, CI, roadmap-gate and Scorecard badges, the go and license and unaudited and scaffold status badges, the maintainer table, the rule list, the what-it-does-not-check section and the roadmap rule.)
- [x] **1.11** Copy the doc set into the repo root. S. Deps: 1.1. Done when: merged. Root copies are prepared in the scaffold; merge is pending. (done 2026-10-30: `PRD.md`, `ARCHITECTURE.md`, `ARCHITECTURE_ESSENTIALS.md`, `project_structure.md`, `AGENTS.md`, `CLAUDE.md` and `ROADMAP.md` all resolve on `origin/main`; the originals in `files/` stay local and Git-ignored.)
- [x] **1.12** Owner-authorized scope-first scaffold, independent of unfinished Phase 0 gates (D-09). S. Done when: root brief copies, scoped package/file placeholders, model declarations, presence-aware policy types and a source interface exist; the skeleton builds, model tests pass, and the CLI exits nonzero without auditing. No feature implementation or remote protection changes are included. (done 2026-10-28: local checks in 1.4, `go run ./cmd/stellar-gauge` prints a scaffold notice and exits 3; merged in PR #1)
- [x] **1.13** Program-submission repo hygiene, from the Stellar Wave playbook Phase 10 (owner-authorized 2026-10-28). S. Deps: 1.1. Done when: CI, roadmap-gate, Dependabot, CODEOWNERS, issue templates, README/CONTRIBUTING/SECURITY rewrites and `docs/banner.svg` are merged; the label taxonomy, repository topics and a planned issue backlog derived from this file exist on GitHub; and the pattern was verified against real approved repos rather than assumed. Remote protection settings are covered by 1.3, a release by 4.8 and a documentation site by a new 5.x task. Done on the remote side (2026-10-30): topics, Discussions, 33 labels, the `protect main` ruleset and 73 issues, #5 to #77, one per open task in Phases 0 to 6 plus the ten readiness gates, generated by `scripts/generate_issues.sh` from this file. Done: the files merged with PR #2 (`ae6a05d`, 2026-10-30) after `ci / go` passed in 48s, `ci / vuln` in 13s and `roadmap-check / gate` in 4s against head `0004f1a`.
- [x] **1.14** Keep the issue backlog in step with this file. `scripts/generate_issues.sh --check` proves every open task in Phases 0 to 6 and every gate has a curated row; `--apply` opens only the issues that are missing, keyed on a marker line in each body. S. Deps: 1.13. Done when: `roadmap-check.yml` runs `scripts/generate_issues.sh --check` and a test PR that adds a task without a backlog row fails. (done 2026-10-30: the step is in `roadmap-check.yml` and PR #78, which added task 1.16 with no row while keeping the progress table correct, failed `roadmap-check / gate` in 7s on that step with `generate_issues: open tasks with no backlog row: 1.16`. The ROADMAP-touched and table steps passed first, so the failure is the backlog row, and the throwaway PR was closed unmerged.)
- [ ] **1.15** Keep `docs/WAVE_SUBMISSION.md` current: the verified program facts, the gate list to clear before applying, the maintainer runbook for a cycle, and the draft submission answers. M. Deps: 1.13. Done when: the file's facts are re-checked and its numbers recounted within the wave cycle the application is filed in, with the check date written at the top.

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
| D-01 | Project and repo name | Decided 2026-10-30 | Working product name Gauge, binary `stellar-gauge`, module `github.com/posture-sec/guage` in the existing repository. The collision evidence from 0.11 is recorded under D-15, which is about the repository slug, not the name in use |
| D-02 | Host organization | Decided 2026-10-28: `posture-sec` | Owner approved the existing origin; module `github.com/posture-sec/guage`. Observed 2026-10-30: `gh api orgs/posture-sec/repos` lists one repository, `guage` |
| D-03 | License | Decided 2026-10-28: Apache-2.0 | Owner approved retaining the existing LICENSE |
| D-04 | Sequencing against Airlock | Decided 2026-10-30: proceed with Gauge features now | Owner chose this after the 0.5 search found no tool duplicating the job. Two facts made waiting pointless rather than cautious: `gh api orgs/posture-sec/repos` returns only `guage`, so there is no public Airlock release whose v0.1.0 could be a precondition, and the shared output is conventions, which are already copied into these documents. Cost: R-2 (maintainer overload across projects) stays High |
| D-05 | Data source | Confirmed 2026-10-30: Horizon first behind the `Source` interface | 0.9 observed that Horizon is not marked deprecated in the developer docs and that Stellar RPC `getLedgerEntries` returns a classic account ledger entry, so an RPC implementation is a later second source rather than a rewrite. RPC stays in Phase 7 |
| D-06 | TOML parser | Confirmed 2026-10-30: `github.com/BurntSushi/toml` v1.6.0 | 0.13 read it from the repository: MIT, no `require` directives in its own `go.mod`, so zero transitive dependencies, last push 2026-08-18, not archived. Owner approved adding it on 2026-10-30 |
| D-07 | Funding or program applications | Open | After v0.2.0 and adoption evidence |
| D-08 | Roadmap gate has no exemptions, including dependency bumps | Decided 2026-10-28 | Matches the owner's instruction for Airlock; assumed to apply here. Dependabot is grouped weekly |
| D-09 | Scope-first scaffold before feature development | Decided 2026-10-28 | Owner approved task 1.12 as a narrow exception to the no-code gate: root documents, placeholders and draft data models only. Demand, protocol verification and feature gates remain open; no deployment or repository-protection changes |
| D-10 | Scaffold toolchain | Decided 2026-10-28 | Go 1.27.3 returned by the official stable-download metadata and verified locally with `go version`; no third-party runtime module added |
| D-11 | Adopt the Stellar Wave playbook's Phase 10 hygiene for this repo | Decided 2026-10-28 | Owner instruction. Pattern verified against `soroauth/soroauth-go` and `plimsoll-protocol/plimsoll-indexer`, not recalled. Its contract and app phases (4 to 9) do not apply to a Go CLI with no Soroban contract, so they are skipped rather than adapted |
| D-12 | No release tag until an auditing build exists | Decided 2026-10-28 | The playbook's Phase 10 item 6 asks for a `v0.1.0` tag. Tagging the scaffold would advertise a release that audits nothing, so the tag stays with task 4.8 |
| D-13 | Do not apply to a Drips Wave until the gate list clears | Decided 2026-10-30 | The program docs say a rejected repository cannot be re-applied and that appeals are capped at three with waiting periods, and no Stellar Wave is open as of 2026-10-30. Spending the first application on a repository whose CLI audits nothing trades a permanent strike for nothing. See `docs/WAVE_SUBMISSION.md` |
| D-14 | Accept the three Branch-Protection warnings while the repository has one maintainer | Decided 2026-10-30 | Scorecard warns that `main` requires no approvers, no CODEOWNERS review and no last-push approval. Requiring one approval would make every pull request unmergeable, because a pull request author cannot approve their own change and there is no second account. The checks that do bind are pinned: PR requirement, three status checks, no force push, no deletion, up-to-date branches. Revisit when a second maintainer joins |
| D-15 | Rename the repository slug away from `guage` before public launch | Proposed 2026-10-30 | 0.11 evidence, all read from GitHub search and the repository API on 2026-10-30: `evoziosk/guage` is an active Python project with the same spelling (4 stars, pushed 2026-10-13), the GitHub account name `guage` is taken (0 public repositories, last touched 2019), and a search for `guage` returns `npm/gauge` before anything here. `getgauge/gauge` (3,195 stars, pushed 2026-10-29) already owns the correctly spelled word in Go. Cost of renaming: the module path, every recorded URL and the issues already generated from this file. Needs the owner's decision, so it is not a blocker on feature work |
| D-16 | Phase 0 verification runs against testnet only | Decided 2026-10-30 | Owner instruction on 2026-10-30. Friendbot funding and reads go to `horizon-testnet.stellar.org` and `soroban-testnet.stellar.org`; mainnet endpoints are quoted from official documentation and marked as not observed. Consequence: fixtures derived from testnet are perishable (testnet resets), and any mainnet-specific shape difference remains unverified |

## 8. Risk register

| ID | Risk | Rating | Mitigation or task |
|---|---|---|---|
| R-1 | Nobody wants it | Medium | 0.4 before feature implementation; owner-authorized scaffold-only exception D-09 |
| R-2 | Maintainer overload across three projects | High | D-04, 0.3; Phase 7 closed |
| R-3 | Threshold or signer semantics modeled wrong | High | 0.6, 0.7, 2.3 |
| R-4 | Horizon deprecation or reshaping | Low to Medium | 0.9 closed 2026-10-30: not deprecated in the current developer docs, and RPC `getLedgerEntries` returns classic account entries, so ADR-005's interface keeps a second source cheap |
| R-5 | "No findings" read as "safe" | High | 4.3, report footer, wording rule |
| R-6 | Expectations edited to match an attacker's change | Medium | 1.3, 5.2, documented residual risk |
| R-7 | `stellar.toml` fetch abuse | Medium | 3.5 |
| R-8 | Name collision | Medium-High for the repository slug, Low for the binary | 0.11 closed 2026-10-30; evidence and the rename option are in D-15, awaiting the owner |
| R-9 | Secret seed pasted into the CLI | Medium | 2.4 |

## 9. Changelog

Newest first. One row per merged change.

| Date (UTC) | Change | Tasks |
|---|---|---|
| 2026-10-30 | Phase 0 closed on the evidence, and the project is authorised to leave scaffold-only status. Read or run on the day: the SEP-23 suite (10 valid and 20 invalid strkeys) against a stdlib encoder and decoder written here, which passed only after three missing rules were added, and which is the basis for confirming ADR-003; a live `getLedgerEntries` call on Soroban testnet returning a classic account ledger entry (84-byte XDR, `latestLedger` 5466028), which confirms ADR-005's Horizon-first, RPC-later shape; SEP-1 v2.7.0 and SEP-29 read from the spec, naming `VERSION`, `NETWORK_PASSPHRASE`, `SIGNING_KEY`, `HORIZON_URL` and `ACCOUNTS` with a 100 KB cap and a mandatory CORS header, plus a real issuer whose `stellar.toml` answers 200 with `content-type: text/html`; Horizon's documented per-IP 3600 requests/hour and 429, and observed `hal+json` responses, `problem+json` errors, a `Latest-Ledger` header, 405 on HEAD, no rate-limit headers and a `limit` ceiling of 200; a `gh api` read of `BurntSushi/toml` (MIT, v1.6.0, no `require` directives, so no transitive dependencies), approved by the owner as the one third-party module; and the name-collision search that produced D-15. `PRD.md` 2.2 now records the landscape as repositories that exist with their own descriptions, led by `signerreach/signerreach`. D-04 is Decided (proceed now; the org has one repository, so there is no public Airlock release to wait for), D-16 records the testnet-only verification policy, and 0.4, 0.6 and 0.7 stay open with the reason each is still open | 0.2, 0.3, 0.5, 0.8, 0.9, 0.10, 0.11, 0.12, 0.13, D-01, D-04, D-05, D-06, D-15, D-16 |
| 2026-10-30 | Scorecard evidence and the rest of Phase 1: the first `scorecard / analysis` run on `main` published overall 6.2 with Branch-Protection scoring 4 rather than erroring, so 1.7 is ticked; 1.6, 1.8, 1.10 and 1.11 are ticked against the files now on `main`, with the Dependabot runs and the roadmap-rule presence in each document observed rather than assumed; D-14 accepts the three approval-related Branch-Protection warnings while the repository has one maintainer | 1.6, 1.7, 1.8, 1.10, 1.11, D-14 |
| 2026-10-30 | OpenSSF Scorecard: `.github/workflows/scorecard.yml` (`ossf/scorecard-action` v2.5.0, `actions/checkout` v7.0.1, `actions/upload-artifact` v7.0.2 and `github/codeql-action` v4.38.3 all pinned by SHA and resolved from their tags before pinning), push and weekly schedules, SARIF to the code scanning dashboard, results published for the badge, and the badge added to the README row. Task 1.7 stays open until the first run on `main` is read | 1.7 |
| 2026-10-30 | Hygiene closeout after PR #2 merged into `main` as `ae6a05d`: the default branch now carries CI, the roadmap gate, Dependabot, CODEOWNERS, issue forms and the rewritten documents, its first push-event CI run passed (`ci / go` 57s, `ci / vuln` 38s), the direct push task 1.3 asks for was refused on `main` itself with "Changes must be made through a pull request", and `scripts/generate_issues.sh --check` became a step in the gate workflow, where throwaway PR #78 proved it by adding a task with no backlog row and failing on that step in 7s | 1.3, 1.13, 1.14 |
| 2026-10-30 | Wave preparation: `scripts/generate_issues.sh` grew `--update` and a wave routing section in every body, the 73 backlog issues were relabelled so `complexity:` now matches the program's Trivial, Medium and High tiers (32, 28, 3, with the 10 gates untiered), the dependency-free research tasks took `good first issue` and `help wanted`, and `docs/WAVE_SUBMISSION.md` records the verified program facts, the gate list and the maintainer runbook | 1.13, 1.15, D-13 |
| 2026-10-30 | Backlog, evidence and enforcement wording: `scripts/generate_issues.sh` opens one issue per open task and gate from this file, idempotent through a marker line in each body, the roadmap gate is proven on both sides (PR #4 failed `roadmap-check / gate` for a change with no roadmap edit, PR #2 passes it), CI is green on the skeleton, and a probe ruleset with the same configuration as `protect main` refused a direct push with "Changes must be made through a pull request" | 1.5, 1.9, 1.13, 1.14 |
| 2026-10-28 | Repo hygiene for program submission: CI and roadmap-gate workflows, gate and progress scripts with an eight-case self-test, Dependabot, CODEOWNERS, issue forms, README/CONTRIBUTING/SECURITY rewritten to the verified approved-repo pattern, banner, plus a `protect main` ruleset, label taxonomy, topics and a roadmap-derived issue backlog | 1.13, 1.3, 1.5, 1.6, 1.8, 1.9, 1.10, D-11, D-12, 5.8 |
| 2026-10-28 | Owner-authorized scaffold: root brief copies, Go module, package and file placeholders, model and policy declarations, source interface, non-operational CLI, README/CONTRIBUTING/SECURITY/PR-template drafts, structure and decision documentation | 1.1, 1.4, 1.8, 1.10, 1.11, 1.12, D-01 to D-04, D-09, D-10 |
| 2026-10-28 | First drafts of the full doc set (PRD, ARCHITECTURE, ESSENTIALS, project_structure, AGENTS, CLAUDE, ROADMAP) with a red-team pass | 0.1 |
