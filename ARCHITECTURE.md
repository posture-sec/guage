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

Validates and classifies Stellar addresses and keys. Used to accept only `G` addresses and to detect secret seeds.

The byte layout is SEP-23 and was written down here on 2026-10-30 from that specification, then checked by running the specification's own test suite (10 valid and 20 invalid strkeys) against an encoder and decoder implemented in standard-library Go: all 30 behave as specified.

- A strkey is one version byte, the payload, and a two-byte CRC16 checksum, encoded as RFC4648 base32 with the standard alphabet and **no padding**. The version occupies the top 5 bits (`version << 3`); the low 3 bits are an algorithm specifier for which only 0 (ed25519 for `PK` types, sha256 for `Hash` types) is defined, so a nonzero specifier is an error, not a warning.
- Verified base values and payload shapes: `G` 6 and `S` 18 and `T` 19 and `X` 23 and `C` 2 and `L` 11 take a 32-byte payload; `M` 12 and `W` 22 append an 8-byte ID **big-endian** after the 32-byte key, so 40; `B` 1 is 33 bytes, a one-byte claimable-balance type (only 0 = v0 is defined) then a 32-byte hash; `P` 15 is 32 key bytes, a 4-byte **big-endian** payload length, that payload, then 0 to 3 zero bytes so the payload plus padding is a multiple of 4. `T`, `S` and `X` have no vectors in the specification, so their 32-byte shape follows from the algorithm table rather than from a test case.
- The checksum is CRC16-CCITT with polynomial x16+x12+x5+1, initial value 0, appended **little-endian**. Getting this backwards is silent: the first implementation here wrote the checksum big-endian and every address it produced was rejected by friendbot as invalid.
- A decoder must reject, before decoding: a length congruent to 1, 3 or 6 mod 8, any `=` padding, and any strkey whose unused trailing bits in the final symbol are nonzero. The last one is the trap that SEP-23 calls out by name, because `base32.StdEncoding.WithPadding(base32.NoPadding)` discards those bits instead of checking them, and the checksum cannot catch it since the discarded bits carry no data. An implementation here accepted `MA7Q...AAAACJUR` until that check was added.
- Therefore: decode, validate the shape and checksum, then **re-encode and require the exact same string**. That is the validity rule the specification states, and it is cheaper to hold than a list of bit-position special cases. The signed-payload type needs one more rule the round trip cannot supply: the length prefix must be no larger than the region it describes, and the zero padding after it at most 3 bytes. A decoder here accepted a `P` vector whose prefix said 20 while 32 bytes followed until that padding bound was written down.
- The split matches `project_structure.md`: `address.go` classifies and decodes, `checksum.go` holds the CRC16 routine, `seed.go` reports whether a string is an `S` address so the policy and CLI layers can reject it with exit 2 without echoing the value. Nothing in this package returns a partially validated address: an error means the caller gets no payload, and every error is a sentinel without the input in its text. Implemented 2026-10-30 (task 2.2): all 10 valid and 20 invalid SEP-23 vectors pass, and the fuzz target ran 676129 executions without a failure.

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

Horizon implementation, with the field shapes captured from `https://horizon-testnet.stellar.org` on 2026-10-30 (task 0.6 commits the fixtures; these are the observed facts the parser is written against):

- `GET {endpoint}/` for the network passphrase and current ledger; the root carries `network_passphrase`, `horizon_version`, `core_version`, `history_latest_ledger`, `ingest_latest_ledger`, `core_latest_ledger` and `current_protocol_version`. Testnet's passphrase string is `Test SDF Network ; September 2015`.
- `GET {endpoint}/accounts/{id}` returns `application/hal+json; charset=utf-8` with `account_id` and `sequence` as **strings** (a sequence passes 2^53, so JSON numbers would lose digits), `subentry_count` and `last_modified_ledger` as numbers, `last_modified_time`, `thresholds` as `low_threshold` / `med_threshold` / `high_threshold`, `flags` as `auth_required` / `auth_revocable` / `auth_immutable` / `auth_clawback_enabled`, `balances` as an array, `signers` as an array of `{weight, key, type}`, `data` as an object, and `home_domain` and `sponsor` only when set. A freshly funded testnet account shows `thresholds` all 0, one signer, and `data` as an empty object rather than absent.
- **The master key is listed inside `signers`** as `{"weight":1,"key":"<the same account_id>","type":"ed25519_public_key"}`. This was the open question in `ARCHITECTURE.md` 15 item 1 and it decides the parser: `sig-no-signing-weight` and the minimum-signer math read the `signers` array alone, with no synthetic master entry added.
- Mixed types on purpose, so decode into typed fields rather than a loose map: balances are decimal strings (`"10000.0000000"`), `sequence` is a string, weights and thresholds are small JSON numbers. Nothing monetary becomes a `float64`. Gauge only compares balances for the facts section, but string equality is still the wrong comparison for a decimal, so parse to a fixed-point value or print the string as given.
- There is **no `/accounts/{id}/balances` route** on the observed deployment: it answers 404 `Resource Missing`. Balances come only from the array embedded in the account object, and whether Horizon truncates that array for an account with many trustlines is still unverified (no captured account had more than one balance).
- Errors are `application/problem+json` with `type`, `title`, `status`, `detail` and `extras.{invalid_field,reason}`. A malformed address gives 400 `bad_request` with reason "Account ID must start with `G` and contain 56 alphanum characters"; a well-formed unknown account gives 404, which is what `ErrNotFound` maps to. Collection endpoints refuse `limit` above 200 and `/accounts` requires exactly one filter.
- A successful account read carries a `Latest-Ledger` header, which is a cheap staleness signal to print alongside the snapshot's `last_modified_ledger`.
- `HEAD` requests are refused with 405, and **no rate-limit or remaining-quota header is returned at all**, so retry logic keys on the 429 status itself; honoring `Retry-After` is best-effort because the observed deployment does not send it.
- One shared `http.Client` with a per-request timeout, a response size cap enforced with `io.LimitReader` plus an explicit overflow check, strict decode into typed structs, bounded retries with backoff on 429 and 5xx, a bounded worker pool across accounts, and a fixed `User-Agent`. Strict means asymmetric on purpose: the HAL envelope carries `_links` and other keys Gauge ignores, while `thresholds` and `flags` must not contain a key Gauge does not model, because an unrecognized flag name is exactly the shape change that should stop a run rather than pass silently.
- HTTPS only, except loopback in tests. No redirects to other hosts. Response bodies are never logged.
- Stellar RPC stays a Phase 7 item, and 0.9 confirmed the door is open rather than theoretical: `getLedgerEntries` on `https://soroban-testnet.stellar.org` accepted the 40-byte classic account `LedgerKey` (ACCOUNT discriminant, then the `AccountID` key-type union, then the 32-byte key) and returned that account's `LedgerEntry` XDR with its `lastModifiedLedgerSeq`, for an account with no Soroban state at all.

### 6.5 `sep1`

Fetches `https://{home_domain}/.well-known/stellar.toml` for identity checks.

- Validate the domain with a strict hostname check; reject IP literals, ports and userinfo.
- Dial-time guard: resolve, then reject loopback, private, link-local (including the cloud metadata address), unique-local, carrier-grade NAT and unspecified addresses. Check the address actually dialed, not just the first lookup, to resist DNS rebinding.
- HTTPS only, at most 3 redirects, each target re-validated and also HTTPS. SEP-1 v2.7.0 says nothing about redirects or HTTP status codes, so this cap is Gauge's own bound and is documented as such; a server is not required to honor it, and the guard only has to hold for Gauge's request path.
- Size cap 100 KiB, which is SEP-1's own stated maximum rather than a placeholder. A body that reaches the cap without a complete parse is `id-toml-unreachable`. Total timeout, content treated as data, CORS header `Access-Control-Allow-Origin: *` fetched but not required for a CLI run (it matters to browsers, and a missing one is printed as a fact, not a finding).
- Content type is **not** a validity signal. SEP-1 only recommends `text/plain`, and the testnet issuer account captured on 2026-10-30 at home domain `komunitin.org` answers 200 with `content-type: text/html` and a single-page-application HTML body. So the check is: fetch under the guard, then require a strict TOML parse that yields at least one recognized identity key. Anything else is unreachable, and the finding names which step failed (transport, parse, or no recognized keys) so an operator can tell a dead domain from a misconfigured one.
- Parse with the TOML library; extract only the fields the rules use (`NETWORK_PASSPHRASE`, `ACCOUNTS`, plus `SIGNING_KEY`, `HORIZON_URL` and `VERSION` as printed facts; there is no `ACCOUNT_ID` field to look for); sanitize everything printed.

### 6.6 `policy`

- Strict decode of `gauge.toml`: `version` required, unknown keys are errors, duplicate accounts are errors.
- Account IDs must be `G` addresses. A secret seed anywhere in the file aborts with exit 2 and a rotate-the-key message that does not repeat the value.
- `network` is required and must be `mainnet`, `testnet` or a full passphrase. There is no default.
- Role profiles map role to severity adjustments (PRD Appendix A).
- `[[accept]]` entries: `rule`, `account`, `reason` required; `expires` optional; no wildcards; some rules are not acceptable (`acct-missing`, `acct-network-mismatch`, `sig-threshold-unreachable`).
- Implemented 2026-10-30 (task 2.4): decoding uses `github.com/BurntSushi/toml` v1.6.0 with `MetaData.Undecoded`, so an unknown key anywhere in the file is an error, and the file is capped at 1 MiB. A secret seed is found by scanning the raw text for an `S` token that validates as a strkey, so it is rejected even in a comment; the error never contains it. Rejections carry a category (`Error.Kind`) and a field path, which is what the CLI maps to exit 2. Network names resolve to their published passphrases, and any other network value is treated as the passphrase itself.

### 6.7 `rules`

A rule is a pure function `func(ctx RuleContext) []model.Finding`. `RuleContext` carries the snapshot, the account's policy, role profile and identity facts. Rules are registered in a plain slice; there is no plugin or scripting system.

### 6.8 `report` and `sanitize`

- Outputs: text, JSON and (P1) markdown.
- `sanitize` removes C0 and C1 control characters, whole terminal escape sequences, bidirectional override characters and zero-width characters, collapses whitespace, truncates length, and flags non-ASCII hostnames in domains (IDN lookalike risk). Markdown output additionally escapes special characters and places evidence only inside fenced blocks with a fence longer than any backtick run inside it. Implemented 2026-10-30 (task 2.5) with a 256-rune field cap, whole CSI and OSC sequence removal, Unicode tag characters, and fuzz targets for text and fencing.
- Ordering: severity descending, then account, then rule ID. Output is deterministic.

### 6.9 `run`

Orchestrates: load policy, build sources, check network passphrase, fetch snapshots concurrently with a bound, fetch identity data, evaluate rules, apply accepts, produce a `Report`, map to an exit code.

### 6.10 `cmd/stellar-gauge`

Argument parsing and dispatch only. No logic.

## 7. Data models

The scaffold contains plain declarations in `internal/model`, policy input types in `internal/policy`, and the `Source` interface. As of 2026-10-30, `internal/model` also carries the severity names, the finding constructor with its fingerprint and the sort order, `internal/strkey` validates and classifies addresses, and `internal/sanitize` cleans output. Signer normalization, threshold math, source parsing and report rendering are still pending. JSON tags are draft snake_case field names; severity is numeric in raw struct encoding. A stable public report schema and serialization behavior remain part of task 2.6. Signer type values (`ed25519`, `hash_x`, `pre_auth_tx`) are internal labels, not a verified Horizon wire mapping.

`policy.Config` holds version, network, endpoints, accounts and accepts. `policy.Account` holds identity, name, role, expected signers, thresholds, flags, home domain and issuer lock intent. Pointer fields preserve omitted versus explicit empty/zero/false expectations. `policy.Acceptance` carries rule, account, reason and an optional expiry string; strict decoding, date validation and non-acceptable-rule enforcement are implemented, while role severity adjustments remain task 3.4. `github.com/BurntSushi/toml` v1.6.0 is the only module in `go.mod`, approved as ADR-004.

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
| ADR-003 | Own strkey package, not the Stellar SDK | Confirmed 2026-10-30. SEP-23's full test suite (10 valid, 20 invalid) passes against an implementation written here in standard-library Go, so the routine is small enough to own | Cost is real but bounded: the rejection rules in section 6.2 are ours to keep correct, and the SDK would have carried a large dependency tree for the same dozen checks |
| ADR-004 | One third-party dependency: a TOML parser | Confirmed 2026-10-30 for `github.com/BurntSushi/toml` v1.6.0 after task 0.13: MIT, no `require` directives in its own `go.mod`, so nothing arrives transitively, last pushed 2026-08-18 and not archived. Go has no TOML | A dependency to watch; Dependabot covers it from the first `go.sum` |
| ADR-005 | `Source` interface with Horizon first | Confirmed 2026-10-30 by task 0.9: Horizon is not marked deprecated in the current developer docs, and a live Soroban RPC `getLedgerEntries` call returned a classic account ledger entry, so a second implementation is possible without changing the interface | A second implementation later, and the ledger-entry XDR is a harder parser than JSON |
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

Design-review analysis. The scenarios below are not tests, and none of them has been exercised except where a row says so; the Phase 0 observations of 2026-10-30 are recorded in the rows they settle. External-system claims that Phase 0 did not touch remain [Unverified].

### 14.1 What would break

| ID | Scenario | Why it breaks | L / I | Resolution | Task |
|---|---|---|---|---|---|
| GB1 | Attacker edits `gauge.toml` and the scheduled workflow to match the changed account | Expectations are policy stored with the code | M / H | Protected branch, CODEOWNERS, baseline stored outside the repo (P1), document the residual risk | 1.3, 5.2 |
| GB2 | Endpoint lies, lags or vanishes | Single source of truth | M / M | Fail closed on errors; two-endpoint comparison; endpoint ledger in output | 2.7, 5.1 |
| GB3 | Operators never write a config | Zero config only gives universal rules | H / M | Useful defaults, facts report, `snapshot` to bootstrap expectations with a loud review banner | 3.7, 5.2 |
| GB4 | "No findings" is read as "safe" | Key custody and key-holder identity are invisible on-chain | H / H | "Posture" wording, an explicit limits section in the README and every report footer | 4.3 |
| GB5 | Threshold math is wrong at the edges | Semantics are subtle (threshold 0, weight 0 master, leftover signatures) | M / H | Pure function, brute-force cross-check, testnet experiments | 0.7, 2.3 |
| GB6 | Master key not present in `signers` (or present with different shape) | Parsing assumption | M / H | **Closed 2026-10-30**: the master key is present as an ordinary signer entry, observed on testnet; the parser reads `signers` alone | 0.6, 2.7 |
| GB7 | Secret seed pasted into the CLI | Ends up in shell history | M / H | Reject on sight, never echo, tell the user to treat it as compromised | 2.4 |
| GB8 | `stellar.toml` fetch abused | SSRF, huge body, hostile redirects, DNS rebinding | M / M | Dial-time guard, caps, redirect limits | 3.5 |
| GB9 | Hostile strings break output | ANSI, bidi, markdown and annotation injection | M / M | Sanitizer on every chain-derived string | 2.5 |
| GB10 | Testnet versus mainnet mix-up | Defaults hide the mistake | M / M | Explicit network and passphrase verification | 2.4, 2.7 |
| GB11 | Horizon is deprecated or reshaped | External dependency | M / M | Mitigated 2026-10-30: Horizon is not marked deprecated in the current docs, and RPC `getLedgerEntries` returns classic account entries, so the interface has a real second implementation. Field-shape drift is caught by the strict decode in 6.4 | 0.9, 7.1 |
| GB12 | Rate limits on public endpoints | Many accounts, parallel fetch | M / L | Measured 2026-10-30: per-IP, 3600 per hour by default, 429 on exceed, no quota headers. Bounded concurrency, backoff on the 429 itself, and a run of 50 accounts costs about 51 requests (`PRD.md` 6.4) | 0.12, 2.7 |
| GB13 | Maintainer overload | Third active project | H / H | D-04 was decided on 2026-10-30 to proceed, so this risk is carried openly rather than sequenced away: Phase 7 stays closed until v0.1.0 and the backlog is labelled for contributors | 0.3, D-04 |

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

Answered in the Phase 0 pass of 2026-10-30, with the answer and where it came from:

1. Does Horizon list the master key inside `signers`, and with what shape? (0.6) **Yes**, observed on testnet: one entry of `{"weight":1,"key":"<account_id>","type":"ed25519_public_key"}`. Section 6.4 records the shape. The fixtures that pin it in `testdata/` are still to be committed, so 0.6 stays open until they are.
2. Threshold edge semantics. (0.7) **Still open and partly unanswerable by observation.** The states that matter (master weight 0 with no other signer, leftover signatures) require a transaction submission, which this tool forbids itself. Section 6.3 keeps those edges marked unverified and the math is pinned by the brute-force cross-check in 2.3 rather than by experiment.
3. SEP-1 fields, size limit and redirect rules. (0.8) **Answered**: `VERSION`, `NETWORK_PASSPHRASE`, `SIGNING_KEY`, `HORIZON_URL`, `ACCOUNTS`; 100 KB; no `ACCOUNT_ID`; no redirect rules in the spec, so the cap in 6.5 is Gauge's. Text in 6.5.
4. Horizon status and RPC coverage for accounts. (0.9) **Answered**: Horizon is not marked deprecated, and `getLedgerEntries` serves classic account entries. ADR-005 stands as written.
5. Own strkey versus SDK import. (0.10) **Answered**: own package. Section 6.2 carries the layout and the rejection rules the spec's test suite exercises.
6. TOML dependency footprint. (0.13) **Answered**: `BurntSushi/toml` v1.6.0, MIT, zero transitive dependencies. ADR-004.
7. Public endpoint rate limits and which endpoints to default to. (0.12) **Answered**: per-IP, 3600 per hour by default, 429 on exceed, `limit` capped at 200; testnet observed, mainnet documented only. `PRD.md` 6.4.

Still genuinely unknown, and not a Phase 0 task:

- Whether Horizon truncates the embedded `balances` array for an account with many trustlines. No captured account had more than one balance.
- The exact `signers` shape for `hash(x)` and pre-authorised-transaction entries, which cannot be funded into existence through friendbot.
- The maximum signer count in practice (the core ledger entry bounds it; no captured account came close).

## 16. Documentation and roadmap maintenance rule

`ROADMAP.md` must be updated on every contribution (code, rules, tests, docs, CI, configuration, dependency bumps, reverts), by humans and agents alike. The same pull request must tick, untick or add the affected tasks, add a changelog entry, and refresh the progress table and `Last updated` line. Enforcement is the required `roadmap-check / gate` check running `scripts/check_roadmap_touched.sh` plus a PR template checkbox (task 1.9, done). Changes to behavior described here must also update `PRD.md`, `ARCHITECTURE_ESSENTIALS.md` and `project_structure.md` in the same PR.
