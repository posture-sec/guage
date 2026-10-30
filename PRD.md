# PRD: Gauge (working name)

| | |
|---|---|
| Status | Draft v0.1 (includes a red-team pass, section 15) |
| Date | 2026-10-28 |
| Owner | Ademola |
| Working name | Gauge. Binary `stellar-gauge`. Scaffold repository `posture-sec/guage`, Apache-2.0; final product naming remains open (section 13) |
| Language | Go |
| Sibling project | Airlock (`airlock-sec/airlock`): a Python scanner for Stellar and Soroban repositories. Gauge audits on-chain account configuration. They share conventions, not code |
| Companion docs | `ARCHITECTURE.md`, `ARCHITECTURE_ESSENTIALS.md`, `ROADMAP.md`, `project_structure.md`, `AGENTS.md`, `CLAUDE.md` |

Conventions: **[Verified]** was checked against a source during research. **[Unverified]** is believed but not checked, or untested. Every Unverified item is repeated in section 14 so none silently becomes a requirement.

> **Roadmap rule (every contribution):** any change to this repository, by a human or an agent, must update `ROADMAP.md` in the same pull request: tick or add tasks, add a changelog entry, refresh the progress table and the `Last updated` line. See section 16.

---

## 1. Summary

Gauge is a read-only command-line auditor for the configuration of Stellar accounts: issuer, distribution, treasury and merchant receiving accounts. It fetches an account's signers, thresholds, flags and home domain, compares them with what the operator declared they should be, and reports misconfigurations and drift.

It is **state-based, not event-based**. It does not watch a stream and alert; it answers "is this account configured the way you think it is, right now?" and can be run on a schedule to catch changes that an event watcher missed.

## 2. Problem

### 2.1 Account configuration is where Stellar funds are lost quietly

- Stellar authorization is a weight sum: the weights of the signatures on a transaction must reach the threshold for each operation. Thresholds default to 0 and the master key defaults to weight 1. [Verified: Stellar docs]
- A mistake can lock an account for good. Docs warn that setting the master key weight to 0 may permanently lock you out if no other signers exist, and that thresholds above the combined signer weights are unreachable. [Verified: Stellar docs]
- Adding or removing signers and changing thresholds is a high-threshold operation (SetOptions). If a single key can reach the high threshold, one compromised key rewrites the account's control. [Verified: SetOptions and AccountMerge are the high-threshold operations per SDK documentation; Unverified: exact wording in core docs]
- Issuer flags carry real power over holders: clawback lets an issuer recall a holder's balance of an asset, and revocation can freeze holders. Clawback must be enabled before issuance. [Verified: Stellar docs excerpts]
- Signers can be `hash(x)` or pre-authorized transaction types in addition to ed25519 keys. A `hash(x)` signer can be satisfied by anyone who knows the preimage. [Verified: signer types exist; Likely: the preimage consequence]

### 2.2 What exists today

- Event and contract monitors are plentiful: Sentinel, OpenZeppelin Monitor, Blip, SoroBeacon and others. [Verified: search results] They watch activity; none that I found audits an account's posture against declared intent. [Unverified: three searches only]
- Webacy offers a beta risk API for third-party Stellar addresses and tokens. [Verified] That judges other people's addresses. Gauge audits the operator's own.
- Stellar Laboratory and explorers display account data, but nothing compares it with an expected policy. [Unverified]

### 2.3 Gap this project targets

Operators of small issuers, treasuries and merchant accounts (the first users are Accensa and its merchants) have no cheap way to answer: who can move funds from this account, how many keys does it take, did anything change, and does the account tell the truth about who owns it (home domain and `stellar.toml`)?

## 3. Users

### 3.1 Primary: small teams operating Stellar accounts

- 1 to 10 people running an issuer, distribution account, treasury or merchant receiving account.
- Not security specialists. They want a report they can act on, and a scheduled check that goes red when something changes.
- First users: Accensa's own accounts and those of its merchants. [Assumption: confirmed by task 0.4]

Jobs to be done:

- "Tell me if this account can be locked or taken over by one key."
- "Tell me if the signers, thresholds or flags differ from what we agreed."
- "Tell me if the account's public identity (home domain and `stellar.toml`) is set up and consistent."
- "Give me something I can show a partner, auditor or program reviewer."

### 3.2 Secondary

DAOs and multisig treasury holders; anchors; grant and program reviewers who want evidence of operational hygiene.

### 3.3 Not for

Soroban contract logic auditing (Scout, Sunbeam and others), end-user wallet protection, third-party address reputation (Webacy and explorers), real-time transaction alerting, exchanges and custodians with their own controls.

## 4. Goals and non-goals

### Goals

- G1. Make the control structure of an account legible: who can authorize what, and with how many keys.
- G2. Catch dangerous or locked-out configurations before they bite.
- G3. Detect drift from a declared policy and from a saved baseline.
- G4. Be safe to run anywhere: read-only, no secrets, bounded network behavior.
- G5. Be installable and useful in under five minutes with zero configuration.

### Non-goals

- Holding, requesting or checking secret keys. Gauge never needs them.
- Submitting transactions or fixing accounts.
- Real-time streaming, alerting integrations or dashboards (state-based cron checks instead).
- Reputation scoring of third-party addresses and assets.
- Soroban contract-account (`C...`) auditing and contract upgrade analysis.
- Verifying where keys are physically held, who holds them or whether two keys belong to one person. On-chain posture cannot see that (section 15, GB4).

## 5. Principles

1. **Posture, not security.** Output says what the chain shows. It never says an account is "secure".
2. **State over events.** Compare the current account state with declared and saved expectations.
3. **Facts first, opinions second.** Every report starts with the plain facts, then findings.
4. **Fail closed.** Unreachable source, unknown network, missing account or malformed data is a finding or an error, never a pass.
5. **Explicit network.** No silent default to mainnet.
6. **Never touch secrets.** A pasted secret seed is rejected and never echoed.
7. **Untrusted inputs everywhere.** Chain data, `stellar.toml` content and endpoint responses are all treated as hostile strings.
8. **Minimal dependencies.** Standard library plus one vetted TOML parser.

## 6. Product requirements

Priority: **P0** ships in v0.1.0, **P1** in v0.2.0, **Later** is backlog. Task IDs refer to `ROADMAP.md`.

| ID | Requirement | Priority | Acceptance criteria |
|---|---|---|---|
| FR-01 | `check`: audit one or more accounts with universal rules, no config needed | P0 | Works for any `G...` account on an explicit `--network` |
| FR-02 | `audit`: audit accounts declared in `gauge.toml` with roles and expectations | P0 | Drift from declared signers, thresholds and flags is reported |
| FR-03 | Horizon source behind a `Source` interface | P0 | A fake source drives all tests; Horizon is one implementation |
| FR-04 | Exact threshold math as a pure, exhaustively tested function (reachability, minimum signers) | P0 | Matches fixtures captured from real accounts and testnet experiments (task 0.7) |
| FR-05 | Signer, flag, account and network rules (6.2) | P0 | Each rule has a positive and a negative fixture |
| FR-06 | Role profiles: `issuer`, `treasury`, `distribution`, `merchant`, `other` change severities (Appendix A) | P0 | Same account yields different severity per role |
| FR-07 | Accepted risks: `[[accept]]` entries with `rule`, `account`, `reason`, optional `expires` | P0 | Accepting never hides a non-acceptable rule |
| FR-08 | Reject secret seeds in any input; never echo them | P0 | A valid `S...` string exits 2 with a rotate-the-key warning |
| FR-09 | Outputs: text and JSON; sanitized strings | P0 | Control and bidi characters are stripped from all chain-derived text |
| FR-10 | Exit codes: 0 pass, 1 findings at or above `fail_on`, 2 config or usage error, 3 source or internal error | P0 | Unreachable endpoint is exit 3, never 0 |
| FR-11 | HTTP hygiene: timeouts, body size caps, bounded retries, bounded concurrency | P0 | Oversized or slow responses fail closed |
| FR-12 | `stellar.toml` identity rules with an SSRF-guarded fetcher | P0 | Private, loopback and link-local targets are refused |
| FR-13 | Roadmap gate: `scripts/check_roadmap_touched.sh`, PR template checkbox, required CI check | P0 | A PR that changes any file but not `ROADMAP.md` fails |
| FR-14 | Multi-endpoint comparison (`src-disagreement`) | P1 | Two endpoints that disagree raise HIGH |
| FR-15 | `snapshot` and `diff` against a saved baseline | P1 | Added or removed signers and changed thresholds are listed |
| FR-16 | Markdown output | P1 | Renders in a GitHub step summary safely |
| FR-17 | Scheduled GitHub Action template that opens an issue on findings | P1 | Drift triggers a visible failure |
| FR-18 | Trustline exposure facts (issuer flags per held asset) | P1 | Facts only, no scoring |
| FR-19 | SEP-29 memo-required check for merchant receiving accounts | P1 | Depends on task 0.8 verification |
| FR-20 | Stellar RPC source implementation | Later | After Horizon's long-term status is clear (task 0.9) |
| FR-21 | Claimable balances, liquidity pool and sponsorship analysis | Later | |
| FR-22 | Alert integrations (chat, email) | Later, probably never | Use JSON output and CI instead |

### 6.1 Rule catalog (v0.1)

Severities shown are defaults; roles change them (Appendix A).

| Rule | Condition | Default severity |
|---|---|---|
| `acct-missing` | Account returns not found (never created, or merged) | HIGH |
| `acct-network-mismatch` | Endpoint's network passphrase differs from the configured network | HIGH |
| `sig-no-signing-weight` | No signer has weight above 0, so the account can never sign again | HIGH (INFO if the issuer declares `expect_locked`) |
| `sig-threshold-unreachable` | Total signer weight is below a threshold, so that tier can never authorize | HIGH |
| `sig-threshold-order` | Thresholds do not satisfy low <= medium <= high (the documented recommendation) | WARN |
| `sig-single-key-high` | One signer alone reaches the high threshold | by role: HIGH (issuer, treasury), WARN (merchant), INFO (other) |
| `sig-hashx` | A `hash(x)` signer exists; HIGH if it alone reaches the medium threshold | WARN or HIGH |
| `sig-unexpected-signer` | A signer is not in `expected_signers` (only when declared) | by role |
| `sig-missing-expected` | A declared signer is absent | HIGH |
| `sig-weight-drift` | Declared weight differs from actual | by role |
| `thr-drift` | Declared thresholds differ from actual | by role |
| `flag-drift` | Declared flags differ from actual | by role |
| `flag-undeclared-clawback` | Clawback is enabled and the operator did not declare it | WARN (HIGH for issuer) |
| `id-no-home-domain` | No home domain on an account whose role expects one | WARN |
| `id-toml-unreachable` | `stellar.toml` cannot be fetched or parsed under the guard | WARN |
| `id-toml-account-unlisted` | The account is not listed in `stellar.toml` | WARN |

Facts always printed: network and ledger as of, signers (type, weight, sponsor), thresholds, flags, minimum signers to reach each tier, home domain and sponsorship. [Unverified: exact `stellar.toml` field names (task 0.8)]

### 6.2 CLI

```
stellar-gauge check --network mainnet|testnet|<passphrase> [--endpoint URL]... [--format text|json] G... [G...]
stellar-gauge audit --config gauge.toml [--format text|json|markdown] [--fail-on high|warn]
stellar-gauge snapshot --config gauge.toml --out baseline.json        (P1)
stellar-gauge diff --config gauge.toml --baseline baseline.json       (P1)
stellar-gauge version
```

### 6.3 Configuration (`gauge.toml`)

```toml
version = 1
network = "mainnet"                      # required; no default
endpoints = ["https://horizon.stellar.org"]

[[account]]
id   = "G..."                            # G addresses only
name = "treasury"
role = "treasury"                        # issuer | treasury | distribution | merchant | other
expected_signers = [ { key = "G...", weight = 1 }, { key = "G...", weight = 1 } ]
expected_thresholds = { low = 2, medium = 2, high = 2 }
expected_flags = { auth_required = false, auth_revocable = false, auth_immutable = false, clawback = false }
home_domain = "example.com"

[[accept]]
rule    = "flag-undeclared-clawback"
account = "G..."
reason  = "Regulated asset; clawback disclosed in terms"
expires = "2027-04-27"                   # optional
```

## 7. Non-functional requirements

- **Read-only:** no writes except an explicit `--out`. No secrets read from environment or files.
- **Determinism:** same snapshot and policy yield identical output, ordered severity first, then account, then rule.
- **Bounded network:** per-request timeout, response size caps, at most a few retries with backoff, bounded concurrency. [Targets, unmeasured]
- **Single static binary** for Linux, macOS and Windows.
- **Dependencies:** standard library plus one TOML parser. Anything else needs owner approval.
- **No telemetry.**
- **Tool supply chain:** pinned CI actions, `govulncheck` clean, attested releases, Scorecard target 8 or higher. [Targets]
- **Roadmap currency:** `ROADMAP.md` reflects reality after every merged change.

## 8. Success metrics

| Metric | Target |
|---|---|
| Synthetic misconfiguration fixtures detected | 100% |
| False HIGH findings on real operator accounts once expectations are declared | 0 |
| Time from install to first report | under 5 minutes |
| Operators who say they would run it on a schedule (task 0.4) | at least 3 |
| Merged PRs that left `ROADMAP.md` untouched | 0 |
| Scorecard score of the repo | 8 or higher |

## 9. Milestones

No calendar dates. Phases and exit criteria live in `ROADMAP.md`.

- **v0.1.0:** Phases 0 to 4. **v0.2.0:** Phases 5 and 6. **Backlog:** Phase 7.
- Proposed feature sequencing (decision D-04): start features only after Airlock v0.1.0 (Airlock roadmap task 5.5). The owner authorized a scaffold-only exception (ROADMAP D-09) for structure and data models; Phase 0 validation and feature gates remain pending.

## 10. Risks and mitigations

| Risk | Likelihood | Mitigation |
|---|---|---|
| Nobody wants it: operators do not audit their accounts | Medium | Demand check (0.4) before any code |
| Solo-maintainer overload across Airlock, Gauge and Accensa | High | D-04 sequencing; Phase 7 closed until v0.1.0 |
| Horizon is deprecated or changes shape | Medium [Unverified] | `Source` interface; task 0.9; RPC later |
| Threshold or signer semantics modeled wrong | Medium | Pure function, fixtures from real accounts, testnet experiments (0.6, 0.7) |
| False sense of security | High | Say "posture", list what is not checked, no "secure" wording |
| Expectation config edited to match an attacker's change | Medium | Protect the config like policy: CODEOWNERS, protected branch, baseline stored separately; residual risk documented |
| `stellar.toml` fetch abused (SSRF, huge body, hostile redirects) | Medium | Dial-time IP guard, size cap, redirect limits |
| Name collision (the `gauge` word, an existing Gauge test tool CLI) | Medium [Unverified] | Task 0.11; binary is `stellar-gauge` |
| Overlap with commercial or SCF-funded tools | Low to Medium | Different job: self-audit against declared policy |

## 11. Evidence register

**Verified against sources**

- Weight-sum authorization, default thresholds of 0 and master weight 1, weight and threshold ranges, master weight 0 cannot sign even threshold-0 operations, recommendation low <= medium <= high, extra signers raise minimum balance, hash(x) and pre-auth signer types exist, leftover signatures can make a transaction fail.
- Lockout when thresholds exceed combined signer weights.
- Flags: authorization required, revocable, immutable, clawback enabled; clawback must be set before issuance.
- Horizon account objects expose flags, signers, home domain, sponsor and trustline flags.
- Existing monitoring tools and Webacy's Stellar risk API.

**Unverified or untested** (listed in full in section 14)

## 12. Open questions

1. Who are the first three users and what would make them run Gauge on a schedule? (0.4)
2. Does Horizon list the master key inside `signers`? (0.6)
3. Exact threshold behavior for edge cases (threshold 0 with no weighted signer; leftover signatures). (0.7)
4. SEP-1 field names, size limit and redirect rules. (0.8)
5. Horizon long-term status; is Stellar RPC `getLedgerEntries` enough for accounts? (0.9)
6. Implement strkey ourselves or import the Stellar Go SDK package? (0.10)

## 13. Decisions

| ID | Decision | Status | Notes |
|---|---|---|---|
| D-01 | Project and repo name | Scaffold identity confirmed | Owner approved existing repo `guage`, working product name Gauge, binary `stellar-gauge`; final naming and collision check remain open |
| D-02 | Host organization | Decided 2026-10-28: `posture-sec` | Existing repository; module `github.com/posture-sec/guage` |
| D-03 | License | Decided 2026-10-28: Apache-2.0 | Retain the existing LICENSE |
| D-04 | Sequencing | Feature sequencing remains proposed | Owner authorized only the scope scaffold before validation (ROADMAP D-09); no feature gates waived |
| D-05 | Data source | Proposed: Horizon first, behind an interface | RPC later |
| D-06 | TOML parser | Proposed: `github.com/BurntSushi/toml`, the only third-party dependency | Go has no TOML in the standard library; check its transitive deps in 0.13 |
| D-07 | Funding or program applications | Open | After v0.2.0 and adoption evidence |
| D-08 | Roadmap gate has no exemptions, including dependency bumps | Decided 2026-10-28 | Matches the owner's instruction for Airlock; assumed to apply here |

## 14. Unverified or untested items

- Whether any equivalent self-audit tool exists (only three searches).
- Horizon returns the master key inside `signers`.
- Threshold semantics at the edges (threshold 0, leftover signatures).
- SEP-1 field names, size limit and redirect rules; SEP-29 behavior.
- Horizon's long-term status; Horizon and RPC rate limits on public endpoints.
- Maximum signers per account (believed 20) and Horizon balance pagination for accounts with many trustlines.
- Who would actually run this (demand).
- All timing, size and Scorecard targets.

## 15. Red-team review (summary)

Full tables with task IDs: `ARCHITECTURE.md` section 14.

**What would break:**

- **GB1.** Expectations in `gauge.toml` can be edited to match an attacker's change, so a scheduled check goes green. Same class as Airlock's B2. Mitigation is process (protected branch, CODEOWNERS, separate baseline), not code.
- **GB2.** A single endpoint can lie, lag or vanish. Unreachable is exit 3; two-endpoint comparison is P1.
- **GB4.** "Posture OK" will be read as "safe". Wording and an explicit limits section address it; no code can fix the underlying gap (key custody is off-chain).
- **GB7.** Users will paste secret seeds into the command line, and shell history keeps them. Rejection with a rotate-the-key message is P0.

**Edge cases added:** merged or missing accounts, muxed `M...` and contract `C...` addresses, master weight 0 with no other signers, hash(x) and pre-auth signers, testnet resets, stale endpoints, hostile strings in home domains and data entries, IDN lookalike domains, redirects and private-IP targets for `stellar.toml`.

**Overengineered, cut or deferred:** a streaming watch daemon, alert integrations, a rules language, a web dashboard, Soroban contract analysis, asset reputation scoring, RPC support in v0.1 and two-endpoint quorum in v0.1.

## 16. Documentation and roadmap maintenance rule

`ROADMAP.md` must be updated on every contribution: code, rules, tests, docs, CI, configuration, dependency bumps and reverts, whether the author is a human or an agent. The same pull request must:

1. Tick, untick or add the affected tasks.
2. Add a changelog entry (date, change, task IDs).
3. Refresh the progress table and the `Last updated` line.

Enforcement: the required `roadmap-check / gate` check runs `scripts/check_roadmap_touched.sh` (task 1.9, done), and the PR template carries a checkbox. If you change behavior described here, update `ARCHITECTURE.md`, `ARCHITECTURE_ESSENTIALS.md` and `project_structure.md` in the same PR.

---

## Appendix A: Role profiles

| Role | Typical account | Expectations that raise severity |
|---|---|---|
| `issuer` | Account that issues an asset | Single-key control is HIGH; flag drift is HIGH; unexpected signers are HIGH; home domain expected; `expect_locked` allowed |
| `treasury` | Holds reserves | Single-key control is HIGH; signer and threshold drift are HIGH |
| `distribution` | Moves issued asset to holders | Single-key control is WARN; drift is WARN |
| `merchant` | Receives payments | Single-key control is WARN; drift is WARN; memo-required check (P1) |
| `other` | Anything else | Facts and universal rules only; single-key control is INFO |

## Appendix B: Synthetic fixture scenarios

Each is a captured or hand-built Horizon account JSON plus the expected findings.

1. Default account: master weight 1, thresholds 0. Single-key control at every tier.
2. Master weight 0 and no other signers.
3. High threshold above total signer weight.
4. Thresholds out of order (low above high).
5. A `hash(x)` signer with weight reaching the medium threshold.
6. A pre-auth transaction signer present.
7. Extra signer not in `expected_signers`; expected signer missing.
8. Clawback enabled and undeclared; flags drifted from declared.
9. Home domain missing; `stellar.toml` unreachable; account unlisted.
10. Account not found (merged); network passphrase mismatch.
11. Muxed address, contract address and secret seed as input.
12. Hostile strings: ANSI escapes and bidi controls in a home domain.
