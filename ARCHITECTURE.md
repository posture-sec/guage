# ARCHITECTURE: Gauge (working name)

| | |
|---|---|
| Status | Draft v0.1 target (includes a red-team pass, section 14); scaffold only under ROADMAP 1.12 and D-09 |
| Date | 2026-10-28 |
| Companion docs | `PRD.md`, `ARCHITECTURE_ESSENTIALS.md`, `ROADMAP.md`, `project_structure.md`, `AGENTS.md`, `CLAUDE.md` |
| Tags | **[Verified]** checked against a source; **[Unverified]** believed or designed but untested |

> **Roadmap rule (every contribution):** any change to this repository, by a human or an agent, must update `ROADMAP.md` in the same pull request: tick or add tasks, add a changelog entry, refresh the progress table and the `Last updated` line. See section 16.

---

## 1. Context and constraints

Gauge fetches the configuration of Stellar accounts from a public API, compares it with declared policy, and reports. It runs on a developer machine or in CI.

Hard constraints:

1. **Read-only and secret-free.** No transaction building or submission, no secret keys, no environment-variable credentials.
2. **Everything fetched is hostile.** Endpoint responses, `stellar.toml` files and every string inside them.
3. **Fail closed.** Anything unreachable, malformed, oversized or ambiguous is a finding or an error, never a pass.
4. **State-based.** Compare current state with expectations. No streams, no daemons.
5. **A solo maintainer must be able to own it.** Few packages, one third-party dependency, no services.

## 2. Tech stack

| Concern | Choice | Reason |
|---|---|---|
| Language | Go, current stable release at project start; version pinned in `go.mod` and CI (`go-version-file`) | Single static binary, strong standard library for HTTP and JSON, native fuzzing and race detector. Horizon and the Stellar Go SDK are Go [Likely] |
| HTTP, JSON, hashing, flags | Standard library (`net/http`, `encoding/json`, `crypto/sha256`, `flag`) | No framework; `flag` with hand-rolled subcommands instead of a CLI framework |
| TOML | `github.com/BurntSushi/toml` (the only third-party dependency) | Go has no TOML in the standard library. Check license and transitive dependencies in task 0.13 |
| Strkey (address) validation | Own small package in v0.1, tested against reference vectors; revisit importing the Stellar Go SDK package | The SDK module is large; a 60-line checksum routine avoids its dependency tree (ADR-003) |
| Stellar data | Horizon account endpoint behind a `Source` interface; Stellar RPC later | Horizon's long-term status is uncertain [Unverified] |
| Testing | `go test -race`, `net/http/httptest` fake Horizon, captured JSON fixtures, golden files, native fuzzing | Offline, deterministic |
| Static checks | `gofmt`, `go vet`, `staticcheck`, `govulncheck` | Dev-time and CI |
| Build | `go build -trimpath`, static binaries for Linux, macOS and Windows | Reproducible-leaning builds |
| Release integrity | Tagged releases with build provenance attestations; consumers verify before use | Matches Airlock |
| CI | GitHub Actions, every action pinned by hash, top-level `contents: read` | Dogfoods Airlock |

## 3. Architecture overview

```
   gauge.toml (policy, accepts)          Stellar endpoint(s) (untrusted)         home_domain host (untrusted)
            |                                      |                                      |
            v                                      v                                      v
   +----------------+     +-------------------------------+                +--------------------+
   | policy loader  |     | Source (Horizon impl)         |                | sep1 fetcher       |
   | strict, no     |     | timeouts, size caps, retries  |                | SSRF guard, caps   |
   | secrets        |     +---------------+---------------+                +---------+----------+
   +-------+--------+                     |  AccountSnapshot                         |  Identity facts
           |                              v                                          v
           |                 +---------------------------+                            |
           +---------------> | rules (pure functions)    | <--------------------------+
                             | threshold math, roles     |
                             +-------------+-------------+
                                           | []Finding
                                           v
                             +---------------------------+
                             | accept filter -> report   |  sanitize all chain-derived text
                             | text / json / markdown    |  -> exit code
                             +---------------------------+
```

Rules are pure functions over a snapshot and a policy. All network I/O lives in `source` and `sep1`.

## 4. Trust model

| Input | Trust | Handling |
|---|---|---|
| Operator's `gauge.toml` | Policy. Trusted only as far as the repo and branch it lives in | Strict parsing; unknown keys are errors; protect like code (CODEOWNERS, protected branch) |
| Endpoint response | Untrusted | Size cap, strict JSON decode, schema checks, passphrase check, fail closed on anything odd |
| Chain strings (home domain, data names) | Hostile | Sanitize before any output; never interpolate into markdown or terminals raw |
| `stellar.toml` content and host | Hostile | SSRF-guarded fetch, size cap, redirect limits, parse as data |
| CLI arguments | User input | Reject secret seeds, M and C addresses with clear messages |

Residual risks that code cannot remove:

- An attacker who can edit the policy file and the scheduled workflow can make the check pass. Mitigations are process controls (section 11).
- A single compromised endpoint can lie. Two-endpoint comparison (P1) narrows this.
- Posture is not custody. Key storage and key-holder identity are invisible on-chain.

## 5. Repository layout

Full annotated tree, import rules and ownership: `project_structure.md`. Summary:

```
gauge/
  go.mod  go.sum
  cmd/stellar-gauge/main.go
  internal/{model,strkey,threshold,source,sep1,policy,rules,report,sanitize,run}/
  testdata/{horizon,policy,golden}/
  scripts/{check_roadmap_touched.sh,roadmap_progress.sh}
  .github/{workflows,pull_request_template.md,CODEOWNERS}
```

## 6. Components

### 6.1 `model`

Plain types shared by every package. No logic beyond small constructors and ordering helpers. Imports nothing internal.

### 6.2 `strkey`

Validates and classifies Stellar addresses and keys: `G` (account), `M` (muxed), `C` (contract), `S` (secret seed), `T` (pre-auth), `X` (hash). Used to accept only `G` addresses and to detect secret seeds. Decoding: base32, version byte, payload, CRC16-XModem checksum (little-endian). Tested against reference vectors and fuzzed. [Unverified: byte-level details recalled from memory; verify against reference vectors in task 2.2]

### 6.3 `threshold`

Pure functions, no I/O:

```go
// Reachable reports whether any set of signers can authorize at this threshold.
func Reachable(signers []model.Signer, threshold int) bool

// MinSigners returns the smallest number of signers whose weights reach the threshold.
func MinSigners(signers []model.Signer, threshold int) (n int, ok bool)
```

Semantics, as documented: signature weights are summed and must be greater than or equal to the threshold; thresholds run 0 to 255; weights 1 to 255; a transaction always needs at least one authorizing key; a master key with weight 0 cannot sign even threshold-0 operations. [Verified: Stellar docs] Edge behavior (threshold 0 with no weighted signer, leftover signatures) is [Unverified] until task 0.7 settles it with testnet experiments.

Rules for the math:

- Only signers with weight above 0 participate.
- Ed25519 and `hash(x)` signers participate. Pre-authorized transaction signers do not count toward standing control (they authorize one specific transaction) and are reported separately.
- `MinSigners` takes weights in descending order and accumulates until the threshold is reached. With threshold 0 the answer is 1 if any signer has weight above 0, otherwise not reachable.
- Exhaustive table tests plus a randomized comparison against a brute-force subset search for small signer sets.

### 6.4 `source`

```go
type Source interface {
    Network(ctx context.Context) (model.NetworkInfo, error)            // passphrase, ledger
    Account(ctx context.Context, id string) (model.AccountSnapshot, error) // ErrNotFound for missing accounts
}
```

Horizon implementation:

- `GET {endpoint}/` for the network passphrase and current ledger; `GET {endpoint}/accounts/{id}` for the account. [Likely: field names per Horizon docs; capture real JSON in task 0.6]
- One shared `http.Client` with a per-request timeout, a response size cap enforced with `io.LimitReader` plus an explicit overflow check, strict decode into typed structs, bounded retries with backoff on 429 and 5xx (honoring `Retry-After`), a bounded worker pool across accounts, and a fixed `User-Agent`.
- HTTPS only, except loopback in tests.
- No redirects to other hosts.
- Response bodies are never logged.

### 6.5 `sep1`

Fetches `https://{home_domain}/.well-known/stellar.toml` for identity checks.

- Validate the domain with a strict hostname check; reject IP literals, ports and userinfo.
- Dial-time guard: resolve, then reject loopback, private, link-local (including the cloud metadata address), unique-local, carrier-grade NAT and unspecified addresses. Check the address actually dialed, not just the first lookup, to resist DNS rebinding.
- HTTPS only, at most 3 redirects, each target re-validated and also HTTPS.
- Size cap (placeholder 100 KiB; [Unverified: SEP-1's own limit, task 0.8]), total timeout, content treated as data.
- Parse with the TOML library; extract only the fields the rules use; sanitize everything printed.

### 6.6 `policy`

- Strict decode of `gauge.toml`: `version` required, unknown keys are errors, duplicate accounts are errors.
- Account IDs must be `G` addresses. A secret seed anywhere in the file aborts with exit 2 and a rotate-the-key message that does not repeat the value.
- `network` is required and must be `mainnet`, `testnet` or a full passphrase. There is no default.
- Role profiles map role to severity adjustments (PRD Appendix A).
- `[[accept]]` entries: `rule`, `account`, `reason` required; `expires` optional; no wildcards; some rules are not acceptable (`acct-missing`, `acct-network-mismatch`, `sig-threshold-unreachable`).

### 6.7 `rules`

A rule is a pure function `func(ctx RuleContext) []model.Finding`. `RuleContext` carries the snapshot, the account's policy, role profile and identity facts. Rules are registered in a plain slice; there is no plugin or scripting system.

### 6.8 `report` and `sanitize`

- Outputs: text, JSON and (P1) markdown.
- `sanitize` removes C0 and C1 control characters, ESC sequences, bidirectional override characters and zero-width characters, collapses whitespace, truncates length, and flags non-ASCII hostnames in domains (IDN lookalike risk). Markdown output additionally escapes special characters and places evidence only inside fenced blocks with a fence longer than any backtick run inside it.
- Ordering: severity descending, then account, then rule ID. Output is deterministic.

### 6.9 `run`

Orchestrates: load policy, build sources, check network passphrase, fetch snapshots concurrently with a bound, fetch identity data, evaluate rules, apply accepts, produce a `Report`, map to an exit code.

### 6.10 `cmd/stellar-gauge`

Argument parsing and dispatch only. No logic.

## 7. Data models

The scaffold contains plain declarations in `internal/model`, policy input types in `internal/policy`, and the `Source` interface. It does not validate addresses, normalize signers, calculate thresholds, sanitize strings, generate fingerprints or render reports. JSON tags are draft snake_case field names; severity is numeric in raw struct encoding. A stable public report schema and serialization behavior remain part of task 2.6. Signer type values (`ed25519`, `hash_x`, `pre_auth_tx`) are internal labels, not a verified Horizon wire mapping.

`policy.Config` holds version, network, endpoints, accounts and accepts. `policy.Account` holds identity, name, role, expected signers, thresholds, flags, home domain and issuer lock intent. Pointer fields preserve omitted versus explicit empty/zero/false expectations. `policy.Acceptance` carries rule, account, reason and an optional expiry string; strict decoding, date validation, role adjustments and non-acceptable-rule enforcement remain pending. No TOML dependency has been added.

```go
type Severity int
const ( SevInfo Severity = iota; SevWarn; SevHigh )

type Role string  // issuer, treasury, distribution, merchant, other

type Signer struct {
    Key     string  // G..., or hash / pre-auth identifier
    Type    SignerType // Ed25519, HashX, PreAuthTx
    Weight  int
    Sponsor string  // optional
}

type Thresholds struct{ Low, Medium, High int }

type Flags struct{ AuthRequired, AuthRevocable, AuthImmutable, ClawbackEnabled bool }

type Balance struct {            // P1 exposure facts
    AssetCode, AssetIssuer string
    IsAuthorized, ClawbackEnabled bool
}

type AccountSnapshot struct {
    ID          string
    Network     string  // passphrase
    Endpoint    string
    Ledger      int64   // as-of ledger
    Sequence    string
    Thresholds  Thresholds
    Signers     []Signer
    Flags       Flags
    HomeDomain  string
    Sponsor     string
    Balances    []Balance
}

type Finding struct {
    RuleID      string
    Severity    Severity
    Account     string
    Message     string   // sanitized
    Evidence    string   // sanitized, short
    Fingerprint string   // sha256(ruleID | account | normalized evidence)
    Accepted    bool
}

type NetworkInfo struct {
    Passphrase string
    Ledger     int64
}

type AccountReport struct {
    Snapshot AccountSnapshot
    Findings []Finding
}

type ExitCode int // 0 pass, 1 findings, 2 usage, 3 source/internal error

type Report struct {
    ToolVersion string
    Network     string
    Ledger      int64
    Accounts    []AccountReport
    ExitCode    ExitCode
}
```

Baseline files (P1) are JSON serializations of `AccountSnapshot` with a schema version and a banner stating that a baseline is a record of state, not an approval of it.

## 8. Fail-closed behavior and exit codes

| Situation | Result |
|---|---|
| Endpoint unreachable, times out or returns non-2xx after retries | exit 3 |
| Response over the size cap or fails strict decoding | exit 3 |
| Network passphrase differs from configured network | `acct-network-mismatch` (HIGH) and stop for that endpoint |
| Account not found | `acct-missing` (HIGH) |
| Input is a secret seed | exit 2, value never echoed |
| Input is `M` or `C` address | exit 2 with a clear message |
| Unknown config key, missing `version`, missing `network` | exit 2 |
| `stellar.toml` unreachable, oversized, blocked by the guard or unparsable | `id-toml-unreachable` (WARN) |
| Two endpoints disagree (P1) | `src-disagreement` (HIGH) |
| Rule panics | recovered, reported as a HIGH internal finding, exit 3 |

Exit codes: 0 pass; 1 findings at or above `fail_on`; 2 configuration or usage error; 3 source or internal error. Anything nonzero fails CI.

## 9. Rule specifications

Role severity matrix: `PRD.md` Appendix A. Key definitions:

- **`sig-threshold-unreachable`:** for each tier (low, medium, high), if the sum of participating signer weights is below the threshold, that tier can never authorize. A high tier that is unreachable means signers and thresholds can never change again.
- **`sig-single-key-high`:** `MinSigners(signers, high) == 1`.
- **`sig-hashx`:** any `HashX` signer; HIGH if its weight alone reaches the medium threshold.
- **`sig-no-signing-weight`:** no participating signer. The account cannot sign anything. For an issuer with `expect_locked = true` this is the intended black-hole configuration and is INFO.
- **Drift rules:** compare declared signers, weights, thresholds and flags with the snapshot. Declared values are optional; undeclared fields are only reported as facts.
- **`flag-undeclared-clawback`:** clawback on and no declared expectation. Clawback must be enabled before issuance, so a later change shows up as drift. [Verified: Stellar docs]
- **Identity rules:** `id-no-home-domain`, `id-toml-unreachable`, `id-toml-account-unlisted`. Field names are confirmed in task 0.8.

## 10. Distribution and CI

- Static binaries per platform attached to a GitHub release with provenance attestations; the README documents the verification command.
- `go install` also works but builds from source on the user's machine.
- Scheduled drift checks (P1): a workflow template that runs `audit` on a cron with `permissions: contents: read` (plus `issues: write` only on the job that opens an issue), pinned by SHA, reading the policy from the default branch. The residual risk in section 4 applies.
- No Docker image in v0.1: an extra unpinned layer and a registry trust hop.

## 11. Self-protection of the repository

- Every Action pinned by hash; top-level `permissions: contents: read`; write only on the release job.
- Rulesets: no force pushes, no deletion, required status checks; `CODEOWNERS` on `internal/`, `cmd/`, `.github/`, `scripts/` and `go.mod`.
- Hardware-key 2FA and a protected recovery email for maintainers; org-level 2FA requirement. These matter more than any code.
- `govulncheck`, `staticcheck` and `go vet` run in CI. Dependabot for `gomod` and Actions, grouped weekly; the TOML dependency is reviewed by hand on every bump.
- Airlock scans this repository in CI (dogfooding both ways).
- `scripts/check_roadmap_touched.sh` runs as a required check (section 16).

## 12. Testing strategy

- **Unit tests** for every package, with `-race`.
- **Threshold math:** exhaustive tables plus randomized comparison with a brute-force search.
- **Fake Horizon** via `httptest`: success, 404, 429 with `Retry-After`, 5xx, slow responses, oversized bodies, malformed JSON, wrong passphrase, redirects.
- **Captured fixtures** in `testdata/horizon/` from real mainnet and testnet accounts for each scenario in `PRD.md` Appendix B (task 0.6).
- **Golden files** for text, JSON and markdown output.
- **Fuzzing:** strkey classification, config decoding, SEP-1 parsing, sanitizer.
- **SSRF tests:** loopback, private ranges, metadata address, DNS rebinding simulation, redirect to a private host.
- **Sanitizer tests:** ANSI escapes, bidi controls, zero-width characters, markdown and annotation injection.
- **Roadmap gate test:** the script fails a diff that omits `ROADMAP.md`.
- No test talks to the real network by default; an opt-in tagged test hits testnet.

## 13. Architecture decision records

| ID | Decision | Reason | Cost |
|---|---|---|---|
| ADR-001 | Go, standard library first | Static binary, strong stdlib, native fuzzing | Hand-rolled subcommands |
| ADR-002 | State-based audit, not an event stream | Crowded space; state comparison catches missed events | No real-time alerts |
| ADR-003 | Own strkey package, not the Stellar SDK | Avoid a large dependency tree for a small routine | Must be tested against reference vectors |
| ADR-004 | One third-party dependency: a TOML parser | Go has no TOML | A dependency to watch |
| ADR-005 | `Source` interface with Horizon first | Horizon's future is uncertain | A second implementation later |
| ADR-006 | Explicit `--network`, no default | Testnet versus mainnet confusion is a real foot-gun | Extra typing |
| ADR-007 | Reject secret seeds everywhere | Seeds end up in shell history and logs | None |
| ADR-008 | Declared expectations plus role profiles | Zero-config universal rules, deeper checks when declared | Config to maintain |
| ADR-009 | Accepted risks are explicit and reasoned, not inline markers | Auditability | Slightly more typing |
| ADR-010 | Sanitize every chain-derived string | Terminal and markdown injection | Output differs from raw data |
| ADR-011 | SSRF guard at dial time | DNS rebinding defeats pre-checks | More network code to test |
| ADR-012 | Rules are pure functions, no plugin or rule language | Smaller attack surface and maintenance | Contributors add rules via PR |
| ADR-013 | Roadmap gate in CI | The roadmap rots without enforcement | Friction on tiny PRs |
| ADR-014 | Scope-only scaffold before features (ROADMAP D-09) | Owner requested visible boundaries and draft models before implementation | Placeholders are not completed features; validation gates remain pending |

## 14. Red-team review

Design-review analysis. Nothing here has been run. Each item should become a test or a task. External-system claims are [Unverified] until the Phase 0 tasks settle them.

### 14.1 What would break

| ID | Scenario | Why it breaks | L / I | Resolution | Task |
|---|---|---|---|---|---|
| GB1 | Attacker edits `gauge.toml` and the scheduled workflow to match the changed account | Expectations are policy stored with the code | M / H | Protected branch, CODEOWNERS, baseline stored outside the repo (P1), document the residual risk | 1.3, 5.2 |
| GB2 | Endpoint lies, lags or vanishes | Single source of truth | M / M | Fail closed on errors; two-endpoint comparison; endpoint ledger in output | 2.7, 5.1 |
| GB3 | Operators never write a config | Zero config only gives universal rules | H / M | Useful defaults, facts report, `snapshot` to bootstrap expectations with a loud review banner | 3.7, 5.2 |
| GB4 | "No findings" is read as "safe" | Key custody and key-holder identity are invisible on-chain | H / H | "Posture" wording, an explicit limits section in the README and every report footer | 4.3 |
| GB5 | Threshold math is wrong at the edges | Semantics are subtle (threshold 0, weight 0 master, leftover signatures) | M / H | Pure function, brute-force cross-check, testnet experiments | 0.7, 2.3 |
| GB6 | Master key not present in `signers` (or present with different shape) | Parsing assumption | M / H | Capture real JSON for each scenario before coding the parser | 0.6 |
| GB7 | Secret seed pasted into the CLI | Ends up in shell history | M / H | Reject on sight, never echo, tell the user to treat it as compromised | 2.4 |
| GB8 | `stellar.toml` fetch abused | SSRF, huge body, hostile redirects, DNS rebinding | M / M | Dial-time guard, caps, redirect limits | 3.5 |
| GB9 | Hostile strings break output | ANSI, bidi, markdown and annotation injection | M / M | Sanitizer on every chain-derived string | 2.5 |
| GB10 | Testnet versus mainnet mix-up | Defaults hide the mistake | M / M | Explicit network and passphrase verification | 2.4, 2.7 |
| GB11 | Horizon is deprecated or reshaped | External dependency | M / M | Interface, task 0.9, RPC in backlog | 0.9, 7.1 |
| GB12 | Rate limits on public endpoints | Many accounts, parallel fetch | M / L | Bounded concurrency, backoff, endpoint choice | 0.12, 2.7 |
| GB13 | Maintainer overload | Third active project | H / H | Sequencing decision D-04, Phase 7 closed until v0.1.0 | 0.3 |

### 14.2 Edge cases

| Area | Case | Decision |
|---|---|---|
| Addresses | Muxed `M...`, contract `C...`, secret `S...`, lowercase, whitespace, wrong checksum | Reject with specific messages (2.2, 2.4) |
| Accounts | Not found (never funded or merged); merged since last baseline | `acct-missing` HIGH |
| Signers | Master weight 0 with no signers; weight 0 entries; max signer count; sponsored signers | Handled in the snapshot parser and rules (2.3, 3.1) |
| Signers | `hash(x)` and pre-auth transaction signers | Special-cased in math and rules (2.3, 3.1) |
| Thresholds | Out of order; above total weight; all zero (default) | Rules `sig-threshold-order`, `sig-threshold-unreachable` |
| Flags | Immutable plus clawback; clawback enabled after issuance (impossible per docs) | Drift rule; test (3.2) |
| Network | Custom passphrases, futurenet, testnet resets that delete accounts | Explicit passphrase; testnet fixtures marked as perishable (0.6) |
| Source | Stale endpoint, slow response, huge trustline lists, pagination | Ledger in output, caps, pagination handling confirmed in 0.6 |
| Identity | IDN lookalike domains, trailing dots, ports, redirects, wrong content type, duplicate keys | Hostname validator, guard, strict parsing (3.5) |
| Output | ANSI and bidi controls in strings; very long strings; non-UTF-8 | Sanitizer (2.5) |
| Config | Duplicate accounts, unknown keys, expired accepts, accepts on non-acceptable rules | Strict loader (2.4, 3.4) |
| Time | Expiry dates compared in local time | UTC only |
| Concurrency | Snapshot taken between ledger closes; mixed ledgers across accounts | Ledger recorded per account |
| Platform | Windows path and newline handling for `--out` and config paths | CI matrix (4.4) |

### 14.3 What is overengineered (cut or deferred)

| ID | Item | Verdict | Reason |
|---|---|---|---|
| GO1 | Streaming watch daemon | Cut | Crowded; cron plus state comparison is simpler and catches missed events |
| GO2 | Alert integrations | Cut | JSON output plus CI covers it |
| GO3 | Rules language or plugins | Cut | Pure Go rules, added by PR |
| GO4 | Web dashboard | Cut | No users yet |
| GO5 | Asset and address reputation scoring | Cut | Webacy territory and needs a data moat |
| GO6 | Soroban contract-account auditing | Cut | Other tools cover contracts |
| GO7 | RPC source in v0.1 | Defer | Horizon first; interface keeps the door open |
| GO8 | Two-endpoint quorum in v0.1 | Defer | P1; single endpoint with fail-closed first |
| GO9 | Importing the full Stellar SDK | Cut | Own strkey package suffices |
| GO10 | A CLI framework | Cut | `flag` is enough |
| GO11 | Trustline exposure scoring | Defer | Facts only, in P1 |

### 14.4 Net effect

P0 is a small, fully offline-testable core: threshold math, snapshot parsing, a dozen rules, a strict config, safe output, a guarded identity check and the roadmap gate. Everything that depends on unverified Stellar behavior is gated behind Phase 0 tasks.

## 15. Open technical questions

1. Does Horizon list the master key inside `signers`, and with what shape? (0.6)
2. Threshold edge semantics. (0.7)
3. SEP-1 fields, size limit and redirect rules. (0.8)
4. Horizon status and RPC coverage for accounts. (0.9)
5. Own strkey versus SDK import. (0.10)
6. TOML dependency footprint. (0.13)
7. Public endpoint rate limits and which endpoints to default to. (0.12)

## 16. Documentation and roadmap maintenance rule

`ROADMAP.md` must be updated on every contribution (code, rules, tests, docs, CI, configuration, dependency bumps, reverts), by humans and agents alike. The same pull request must tick, untick or add the affected tasks, add a changelog entry, and refresh the progress table and `Last updated` line. Enforcement is `scripts/check_roadmap_touched.sh` in CI plus a PR template checkbox (task 1.9); until then, reviewers enforce it. Changes to behavior described here must also update `PRD.md`, `ARCHITECTURE_ESSENTIALS.md` and `project_structure.md` in the same PR.
