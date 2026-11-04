# ARCHITECTURE: Gauge (working name)

| | |
|---|---|
| Status | Draft v0.1 target (includes a red-team pass, section 14); functional alpha through Phase 2 plus the account, flag and identity rules as of 2026-11-03, with threshold math and signer rules still pending |
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
| CI | GitHub Actions, every action pinned by hash, top-level `contents: read` | Airlock dogfoods itself; Gauge is meant to run it too, at roadmap 4.7, and does not yet |

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

Semantics, settled against the implementation. Task 0.7 asked four questions about edge behavior and its title proposed answering them on testnet. Submitting a transaction is forbidden here (`AGENTS.md` section 4), and the four questions are deterministic rules the network enforces in code, so they were settled by reading that code. The pin is `stellar-core` **v30.0.0** at commit `0e49751aa9ddf892302cca02cea7f3f5d97df9f1`, which is the build `testdata/horizon/root-testnet.json` names in its own `core_version` field for the endpoint Gauge reads, and every line range below is at that commit. The same functions were then re-read at **v29.0.0** `a9d72b0cac3a89ebbcf926af449ac73089e0f8f7`, the release matching the `current_protocol_version` of 29 that same root records, because a rule that changes between the active protocol and the supported one is a rule a read cannot rely on: `SignatureChecker.h`, `SignatureChecker.cpp`, `SignatureUtils.cpp` and `SetOptionsOpFrame.cpp` are byte-identical across the two, `OperationFrame.cpp` places `getNeededThreshold` and `OperationFrame::checkSignature` at the same lines in both, and only `TransactionFrame.cpp` shifts, by 22 lines around `checkSignature` and 21 around the two error paths, with the statements themselves unchanged. The XDR is read from `stellar/stellar-xdr` v30.0, which is the schema line both protocol versions speak. Method and reasoning are in 0.7's task line; every claim below carries a file and line range so anyone can re-check it at the tag without a network or an account.

What the reference implementation does:

- **Which keys are candidates.** `TransactionFrame::checkSignature` (`src/transactions/TransactionFrame.cpp:428-444`) builds the list as the master key *only when* `acc.thresholds[0]` is nonzero, then appends every entry of the account's `signers` array. A weight-0 master is not a signer of weight zero; it is absent, and nothing downstream can recover it. That is why "a master key with weight 0 cannot sign even threshold-0 operations" holds: it never reaches the comparison.
- **How the sum works.** `SignatureChecker::checkSignature` (`src/transactions/SignatureChecker.cpp:46-169`) adds the weight of each signature it *matches* and returns the moment `totalWeight >= neededWeight` (`:100-110`). It never sums the account's weights and never asks whether an unmatched signature would have helped.
- **Reachability is a comparison against a capped total.** Because a transaction may present every signer it wants and the test is `>=`, the strongest subset is the largest available one, so a tier is reachable exactly when the sum of the top weights it can carry reaches the threshold. The cap is real: the envelope holds `DecoratedSignature signatures<20>` (`Stellar-transaction.x:926-929` for v1, `:969-972` for v0, `:995-998` for a fee bump) and `SignatureChecker` is constructed over `xdr::xvector<DecoratedSignature, 20>` (`SignatureChecker.h:24`, `:61`), while an account may hold `MAX_SIGNERS = 20` entries *plus* a nonzero master (`Stellar-ledger-entries.x:135`, `:206`). A shape with 21 participating weights can therefore present only 20 signatures, and `verifyAll` erases a signer once it has been matched (`SignatureChecker.cpp:111`), so one signature cannot be counted twice. `Reachable` sums the 20 largest participating weights, and that is the whole rule: no subset search.
- **Threshold 0 with nothing to sign with authorizes nothing.** `neededWeight` is the stored threshold and no branch treats 0 specially: `getNeededThreshold` (`src/transactions/OperationFrame.cpp:48-64`) returns `acc.thresholds[level]` verbatim in a `switch` whose `default` calls `abort()`, and the comparison inside `checkSignature` is reachable only from inside a match. With an empty candidate list every loop body is skipped and the function returns false at `:168`. The envelope then fails `txBAD_AUTH` (`TransactionFrame.cpp:1683-1688`) and an operation fails `opBAD_AUTH` (`OperationFrame.cpp:217-238`). The envelope is checked against `THRESHOLD_LOW` (`TransactionFrame.cpp:514-519`); each operation against its own level. So a transaction does need at least one authorizing key, and that key must carry a nonzero weight.
- **Leftover signatures invalidate the transaction.** Each match marks its signature used (`SignatureChecker.cpp:99`) and `checkAllSignaturesUsed` (`:171-191`) returns false if any was never consumed, which the transaction path turns into `txBAD_AUTH_EXTRA` (`TransactionFrame.cpp:1571-1575`). A signature matching no participating key fails the same way a wrong one does, including a signature from a weight-0 master. Over-signing is not harmless, and for a read-only tool the useful consequence is that the participating set is the only thing that matters.
- **`hash(x)` is a hash lock, not a signature.** `SignatureUtils::verifyHashX` (`src/transactions/SignatureUtils.cpp:83-91`) is exactly `signerKey.hashX() == sha256(sig.signature)`, the hint being the last four bytes of that hash (`:107-124` with `:63-81`), and this path is tried first (`SignatureChecker.cpp:120-128`). No public-key operation is involved: the signature field carries the pre-image `x`, and knowing `x` is the only requirement, satisfied at that signer's weight. What PRD 2.1 called "Likely" is now the implementation: spending a `hash(x)` signer puts `x` into a published envelope, so whoever can authorize at that signer has already learned the secret and anyone reading the transaction can too.
- **Pre-authorized transactions match a hash, not a key.** `SignatureChecker.cpp:70-85` compares each `T...` entry against the contents hash of the transaction being checked, which is what makes it good for exactly one transaction and unable to be reused. One asymmetry the doc summaries leave out: that path adds weight **without** marking any signature used, so a `T...` entry can carry a tier on an envelope that presents nothing, and `checkAllSignaturesUsed` over an empty signature list returns true. Whether such an envelope clears every other check is not settled here and is not needed to audit a read.
- **An unreachable tier is a legal stored state.** `SetOptionsOpFrame::doCheckValid` (`src/transactions/SetOptionsOpFrame.cpp:230-328`) validates only that each weight and threshold is at most 255 (`SET_OPTIONS_THRESHOLD_OUT_OF_RANGE`), that a new signer is not the account's own master key (`SET_OPTIONS_BAD_SIGNER`), and from protocol 10 that a signer weight fits one byte. It never compares thresholds with each other and never compares them with the weights available, and `SetOptionsResultCode` (`Stellar-transaction.x:1394-1410`) has no code for an unreachable threshold. `low 5 / med 3 / high 3`, and `high 255` beside a weight-0 master and one signer at weight 1, are states the ledger accepts and holds indefinitely, failing only the operations that need them. This is why `sig-threshold-unreachable` and `sig-threshold-order` are findings about an account rather than about a mutation the protocol refused.
- **Weights run 1 to 255 for every non-master signer**, because `SetOptionsOpFrame::doApply` reads a zero weight as a delete: `if (mSetOptions.signer->weight)` adds or changes, the `else` branch calls `deleteSigner` (`SetOptionsOpFrame.cpp:209-224`). The only zero weight that can exist on the ledger is the master's, and `ARCHITECTURE.md` 6.4 records that Horizon lists the master inside `signers` anyway, which is how `testdata/horizon/7-extra-signer.json` carries its own account id at weight 0 beside five keys at weight 1.
- **Bounds and clamps.** `Signer.weight` is declared `uint32` with the comment "really only need 1 byte" (`Stellar-ledger-entries.x:106-110`), and from protocol 10 a stored weight above 255 is clamped to 255 at check time (`SignatureChecker.cpp:75-80` and `:101-106`).
- **SetOptions is itself a high-threshold operation** whenever it touches a weight, a threshold or a signer (`SetOptionsOpFrame.cpp:30-42`), which is the citation behind PRD 2.1's claim that one key able to reach high can rewrite an account's control.

Two things the reading corrected rather than confirmed, both of them statements this section made before:

- The candidate set is **four** key types, not two. `Stellar-types.x:47-53` lists `ED25519`, `PRE_AUTH_TX`, `HASH_X` and `ED25519_SIGNED_PAYLOAD`, and the last one participates through its own verify path (`SignatureChecker.cpp:151-162`, `SignatureUtils.cpp:48-61`: an ed25519 signature over the signer's stored payload rather than over the transaction). Before task 2.11, `internal/model/signer.go` modeled three and `decodeSigners` refused the fourth with `a signer type Gauge does not model`, so an account carrying a `P...` signer ended a run with a source error and exit 3 instead of a report. Failing closed was the correct behavior for an unknown shape and reporting is the correct behavior for a known one, so 2.11 named the type and taught the parser it: four constants at `internal/model/signer.go:6-9`, the fourth case at `internal/source/horizon.go:162-163`, and the refusal still standing at `:164-165` for anything beyond those four. `Reachable` and `MinSigners` count the type as soon as 2.3 implements them.
- "Pre-authorized transaction signers do not count toward standing control" answers Gauge's question, not core's. Core does add a matching `T...` weight, but only for the one transaction whose hash it holds. The rule stands precisely because a `T...` entry cannot be reused and a `X...` entry is a secret rather than a key, so neither is a way someone *holds* control today.

Rules for the math, each traceable to the block above:

- Only signers with weight above 0 participate. Verified twice over: `TransactionFrame.cpp:436` drops a zero-weight master and `SetOptionsOpFrame.cpp:209-224` makes any other zero weight a delete.
- Ed25519, `hash(x)` and `ed25519_signed_payload` signers participate. Pre-authorized transaction signers do not count toward standing control (each is bound to one transaction hash at `SignatureChecker.cpp:70-85`) and are reported separately.
- `Reachable` sums participating weights in descending order over at most the 20 largest, and compares that against the threshold. The signature cap is why the sum is not simply the account total.
- `MinSigners` takes weights in descending order and accumulates until the threshold is reached. With threshold 0 the answer is 1 if any participating signer exists, otherwise not reachable, because a match is required before any comparison happens (`SignatureChecker.cpp:89-118`, `:168`).
- Exhaustive table tests plus a randomized comparison against a brute-force subset search for small signer sets. The claim the brute-force check exists to test is the greedy one: largest-first minimizes the count subject to `>=`, and the 20-signature cap is part of that search.

Implemented 2026-11-04 by task 2.3: `internal/threshold/threshold.go` holds these rules and no I/O, `participating` drops entries at weight 0 and every `T...` entry and truncates the sorted list at `maxSignatures = 20`, and `threshold_test.go` pairs the named cases (a weight-0 master alone, threshold 0 with and without a match, a `T...` entry that adds nothing, a full account one weight past an envelope) with the randomized subset comparison. The comparison is the check on the greedy claim, not on the cap: its signer sets stay under 20 on purpose because a subset search over 21 elements is the exponential case the cap exists to avoid.

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
- Implemented 2026-10-30 (task 2.7): `NewHorizon` accepts only an https base URL, or plain HTTP for a loopback host, and refuses credentials, paths, queries and fragments. One shared `http.Client` refuses every redirect. Each attempt runs under a per-request timeout inside a semaphore (default eight concurrent), reads at most 1 MiB through `io.LimitReader` plus an overflow check, and never reads the body of a failed response. Transport errors, 429 and 5xx are retried up to three times with exponential backoff capped at 5 s; `Retry-After` is honoured only in integer seconds and capped at 5 s, and a date value falls back to the client's own backoff. `thresholds` and `flags` are decoded strictly, so a missing or unknown key is `ErrDecode`, while the HAL envelope and other top-level keys are ignored on purpose. Signer types map from the four strings Horizon serves, `ed25519_public_key`, `sha256_hash`, `preauth_tx` and `ed25519_signed_payload`, and each key must classify as the matching strkey kind; any other type stops the read. The strings are not read off a documentation page: `signers[].type` is produced by `protocol.MustKeyTypeFromAddress` at `internal/resourceadapter/account_entry.go:92` and `:106` of `stellar/stellar-horizon` at commit `b3ff4119cc99ebcc8048334f5ec64e78c16d1666`, which is the commit `testdata/horizon/root-testnet.json` names in its own `horizon_version` field, and that function looks up `KeyTypeNames` in `protocols/horizon/main.go:25-32` of `stellar/go-stellar-sdk` at tag `v0.8.0`, the version the same commit's `go.mod` requires. `main.go:31` pairs `strkey.VersionByteSignedPayload` with `ed25519_signed_payload`, and `strkey/main.go:40` fixes that version byte as `15 << 3`, the value `internal/strkey/address.go:109` classifies as `P`. The type is therefore known from the implementation, not observed: no account on the captured endpoints carries a `P...` signer, and adding one means submitting a `SetOptions`, which this project forbids itself.
 Balances accept `native`, `credit_alphanum4` and `credit_alphanum12`; a credit balance must name a valid issuer and carry `is_authorized`, which a live testnet holder confirmed on 2026-10-30 (`GA222IF5CAODCVDK7IAACYHQDDERQFZXF66QYIXJ7S347HC3EPWEWLWO`, one USDC trustline, `is_authorized: true`), and `is_clawback_enabled` defaults to false because that capture omits it. Any other asset type stops the read. `Account` leaves `snapshot.Network` empty, so the orchestrator fills it from the root passphrase it has already verified; `CheckPassphrase` is the comparison it calls, and its error names neither passphrase. Tests cover each failure mode against a fake server, and no response body reaches an error.
- Stellar RPC stays a Phase 7 item, and 0.9 confirmed the door is open rather than theoretical: `getLedgerEntries` on `https://soroban-testnet.stellar.org` accepted the 40-byte classic account `LedgerKey` (ACCOUNT discriminant, then the `AccountID` key-type union, then the 32-byte key) and returned that account's `LedgerEntry` XDR with its `lastModifiedLedgerSeq`, for an account with no Soroban state at all.

### 6.5 `sep1`

Fetches `https://{home_domain}/.well-known/stellar.toml` for identity checks.

- Validate the domain with a strict hostname check; reject IP literals, ports and userinfo.
- Dial-time guard: resolve, then reject loopback, private, link-local (including the cloud metadata address), unique-local, carrier-grade NAT and unspecified addresses. Check the address actually dialed, not just the first lookup, to resist DNS rebinding.
- HTTPS only, at most 3 redirects, each target re-validated and also HTTPS. SEP-1 v2.7.0 says nothing about redirects or HTTP status codes, so this cap is Gauge's own bound and is documented as such; a server is not required to honor it, and the guard only has to hold for Gauge's request path.
- Size cap 100 KiB, which is SEP-1's own stated maximum rather than a placeholder. A body that reaches the cap without a complete parse is `id-toml-unreachable`. Total timeout, content treated as data, CORS header `Access-Control-Allow-Origin: *` fetched but not required for a CLI run (it matters to browsers, and a missing one is printed as a fact, not a finding).
- Content type is **not** a validity signal. SEP-1 only recommends `text/plain`, and the testnet issuer account captured on 2026-10-30 at home domain `komunitin.org` answers 200 with `content-type: text/html` and a single-page-application HTML body. So the check is: fetch under the guard, then require a strict TOML parse that yields at least one recognized identity key. Anything else is unreachable, and the finding names which step failed (transport, parse, or no recognized keys) so an operator can tell a dead domain from a misconfigured one.
- Parse with the TOML library; extract only the fields the rules use (`NETWORK_PASSPHRASE`, `ACCOUNTS`, plus `SIGNING_KEY`, `HORIZON_URL` and `VERSION` as printed facts; there is no `ACCOUNT_ID` field to look for); sanitize everything printed.

- Implemented 2026-11-03 (task 3.5): `internal/sep1` is `guard.go`, `fetch.go` and `parse.go`, the second and last package in the tree that touches a socket. `ValidateHostname` refuses IP literals, ports, userinfo, empty or over-long labels, non-ASCII and anything carrying a control character. `ClassifyIP` rejects an explicitly enumerated table (`0.0.0.0/8`, `10/8`, `127/8`, `100.64/10`, `169.254/16`, `172.16/12`, `192.168/16`, `198.18/15`, `224/4`, `240/4`, `::`, `::1`, `fc00::/7`, `fe80::/10`, `ff00::/12`, each IPv4-mapped form unwrapped first so `::ffff:127.0.0.1` cannot smuggle loopback) rather than trusting `IsGlobalUnicast`, which accepts carrier-grade NAT and RFC4193 unique-local and returns true for `::`. A nil address fails closed. The two IPv6 forms that embed an IPv4 address, the well-known NAT64 prefix `64:ff9b::/96` and 6to4 `2002::/16`, are classified as the IPv4 they really reach rather than as a public v6 literal. `HomeDomain` trims surrounding whitespace before validating, because a captured account carries a single space in the `home_domain` field: that is an absent domain, not a hostile one, and it must not become a fetch. That reading lives in one place, `model.AccountSnapshot.HasHomeDomain`, which `run` asks before it decides to fetch and each identity rule asks before it decides to speak: the shape is real mainnet data, `whitespace-home-domain` in `posture-sec/stellar-account-fixtures`, whose own scenario line is "home_domain is a single space, which a tool that trims before validating reads as absent". A check written as `HomeDomain != ""` instead would have sent that space to a resolver and reported an unverified identity claim on an account that publishes nothing.
- The guard is a `http.Transport` whose `DialContext` resolves the name through an injectable `Resolver`, classifies every address in that answer, and then dials the numeric address it validated. The socket therefore always goes where the check looked, including the re-dial after a redirect, which is what a short time-to-live cannot buy. SNI and the `Host` header still come from the URL, so certificate verification is untouched. `Proxy` is nil, because an `HTTPS_PROXY` value would otherwise make the guard validate the proxy instead of the target and that is a total bypass; keep-alives are disabled so every hop re-resolves; one host gets one connection; `MinVersion` is TLS 1.2 and `InsecureSkipVerify` is never set.
- Redirects are followed by Gauge's own loop, `CheckRedirect` returning `http.ErrUseLastResponse`. Each target is re-parsed and required to be https, with no userinfo, no port but 443 and a hostname that passes validation, so it re-enters the guard with fresh resolution; a refused hop is reported as the redirect step and never requested. One `context.WithTimeout` covers the whole chain, so three hops cannot silently quadruple the stated budget.
- Bodies are read through `io.LimitReader` with an explicit overflow check at 102400 bytes, which is the looser of the two readings of SEP-1's "100 KB". Nothing is kept from an oversized body, and the byte count stays unset so a report never implies a document was read.
- Attempts are bounded in the shape `internal/source/http.go` established: at most three per URL, a 200ms exponential backoff, and only a transport or status failure retried. A `Retry-After` from a 429 beats the client's own backoff and is capped at five seconds. 404, 204 and any other non-2xx end the request at the status step rather than retrying into a stall, and the one deadline covers the sleeps too, so a server cannot hold a run past its stated budget.
- `Parse` means strict syntax plus at least one recognized identity key, and it deliberately does not reject unknown keys the way `policy.Load` does: real files carry `ORGANIZATION`, `[Currencies.X]` tables and unrelated keys, and refusing them would mark almost every live domain unreachable. `ACCOUNTS` entries that do not classify as `G` addresses are counted and never carried.
- `Result` carries no `net/http`, `url` or `time` types, so `rules` may read it, and no field of it is ever remote content: the body goes only to the parser, and a blocked address is described by its class rather than its digits. Content type is never inspected, and an absent CORS header is a fact rather than a finding.
- A non-ASCII host is refused at hostname validation and surfaces as `id-toml-unreachable`, not fetched: Go's standard library does no IDNA mapping and `golang.org/x/net/idna` needs owner approval, so `sanitize.Domain`'s IDN flag remains the reporting side of that risk. `sep1` does not import `policy`; the import diagram puts them side by side, and its hostname check is deliberately stricter than `policy.validHomeDomain`, which permits non-ASCII in a declared value. The duplication is intentional, the same way `policy` already mirrors `source`.

### 6.6 `policy`

- Strict decode of `gauge.toml`: `version` required, unknown keys are errors, duplicate accounts are errors.
- Account IDs must be `G` addresses. A secret seed anywhere in the file aborts with exit 2 and a rotate-the-key message that does not repeat the value.
- `network` is required and must be `mainnet`, `testnet` or a full passphrase. There is no default.
- Role profiles map role to severity adjustments (PRD Appendix A).
- `[[accept]]` entries: `rule`, `account`, `reason` required; `expires` optional; no wildcards; some rules are not acceptable (`acct-missing`, `acct-network-mismatch`, `sig-threshold-unreachable`).
- Implemented 2026-10-30 (task 2.4): decoding uses `github.com/BurntSushi/toml` v1.6.0 with `MetaData.Undecoded`, so an unknown key anywhere in the file is an error, and the file is capped at 1 MiB. A secret seed is found by scanning the raw text for an `S` token that validates as a strkey, so it is rejected even in a comment; the error never contains it. Rejections carry a category (`Error.Kind`) and a field path, which is what the CLI maps to exit 2. Network names resolve to their published passphrases, and any other network value is treated as the passphrase itself.
- Endpoint scheme, amended 2026-10-30 during 2.9: `https` anywhere, plain `http` only for a loopback host. The schema now mirrors `internal/source`, which already refused plain HTTP for any other host, so a policy can name every endpoint the client itself accepts and no runtime surprise replaces a config error. The enforcement point is unchanged: `source.normalizeEndpoint` still rejects a non-loopback `http` endpoint before any request is made.

### 6.7 `rules`

A rule is a pure function `func(rules.Context) []model.Finding`. `rules.Context` carries the snapshot, the account's policy, whether the account was missing or the network mismatched, the observed and requested passphrases and, since task 3.5, the `*sep1.Result` the caller fetched: nil means no fetch was attempted. Role severity adjustment is not applied yet (task 3.4), so every rule uses its baseline severity. Rules are registered in a plain slice; there is no plugin or scripting system.

Implemented 2026-10-30 (tasks 3.2 and 3.3) with four rules, evaluated in registry order: `acct-missing` (HIGH) when the endpoint answered 404, `acct-network-mismatch` (HIGH) when the endpoint's passphrase differs from the requested network, `flag-drift` (WARN) comparing only the flags the policy declares, and `flag-undeclared-clawback` (WARN) when clawback is observed on and the policy says nothing about it. A declared clawback value, true or false, suppresses the undeclared warning; a declared false that disagrees with the chain is `flag-drift` instead. Flag comparisons run in a fixed order so evidence is deterministic. Neither account rule can be accepted (section 8), and both suppress every other rule because there is no trustworthy snapshot to compare against. `All()` returns a copy of the registry, and `Evaluate` is the default `run.Evaluator`.

Implemented 2026-11-03 (task 3.5) with the three identity rules, so the registry holds seven: `id-no-home-domain` (WARN) when the policy declares a home domain and the chain shows none, `id-toml-unreachable` (WARN) when a document could not be fetched or parsed under the guard and equally when an observed home domain was never fetched, and `id-toml-account-unlisted` (WARN) when a readable document does not name this account. All three suppress on `Missing` and `Mismatch` like the others, and both toml rules also suppress when the observed field is blank after trimming, because an account that publishes no domain has no identity claim to leave unverified. None of the three joins the non-acceptable list, so an operator can accept a warning about a domain that is down on purpose. Two triggers are narrower than `PRD.md` 6.1 phrases them, deliberately: the role table that says which accounts *expect* a home domain is task 3.4, so `id-no-home-domain` keys on a declaration today and cannot fire in `check`, which declares nothing; and a document naming a different `NETWORK_PASSPHRASE` is carried in the result as a fact rather than raised as a finding, because no rule ID covers it and inventing one or borrowing another's is an owner decision. Purity holds in the direction the diagram allows: `rules` imports `sep1` for its data types only, and no function in `rules` performs I/O, so an identity rule reads what the caller put in its context instead of fetching anything. The cost of putting `Result` in the same package as the client is that `go list -deps ./internal/rules` now shows `net/http` transitively. The check that still means something is the direct one, `go list -f '{{.Imports}}' ./internal/rules`: seven imports, four of them Gauge packages (`model`, `policy`, `sep1` and, since 3.1, `threshold`), no `source`, no `net/http`, and `grep -rn "net/http" internal/rules/` empty.

Implemented 2026-11-04 (task 3.1) with the nine signer and threshold rules, so the registry holds sixteen and `internal/rules/signers.go` and `internal/rules/thresholds.go` are no longer placeholders: `sig-no-signing-weight` (HIGH; INFO for an issuer that declares `expect_locked`, which means this state on purpose), `sig-threshold-unreachable` (HIGH, and on the non-acceptable list in `policy`, so no `[[accept]]` entry can silence it), `sig-threshold-order` (WARN when the tiers do not satisfy low &lt;= medium &lt;= high), `sig-single-key-high` (INFO today, for the reason two paragraphs down), `sig-hashx` (WARN; HIGH when that entry's own weight reaches a nonzero medium), `sig-unexpected-signer` (HIGH), `sig-missing-expected` (HIGH), `sig-weight-drift` (WARN) and `thr-drift` (WARN, comparing only the tiers a policy declares). All nine suppress on `Missing` and `Mismatch` like the first seven, and each is tested in both directions with the guard proved: `TestSignerAndThresholdRulesSkipUnreadAccounts` first asserts the rule fires on a readable account before it asserts an unread one silences it, so a guard that was never reachable cannot pass as tested.

Every one of them calls `internal/threshold` instead of restating the arithmetic. The 20-signature cap, the zero-weight exclusion and the pre-authorization exclusion exist in exactly one place, and a rule that duplicated them could drift from what 0.7 verified line by line; the cap now has a rule-level test as well, `a full account cannot present its 21st signature`, which is `MAX_SIGNERS = 20` plus a master key against `DecoratedSignature signatures<20>`.

Three decisions are worth naming because the task line does not settle them. `sig-single-key-high` ships at INFO rather than the PRD's by-role value: every unconfigured account on the network has thresholds 0/0/0 and one weighted key, `check` declares no role, and `PRD.md` Appendix A's `other` row is the only row that applies before 3.4. A plain `check` therefore prints it on nearly every account it reads, which is why it is INFO and never suppressed rather than a warning that would fail an operator's default run; 3.4 turns the same statement into HIGH for an issuer or treasury and WARN for a merchant or distribution. Second, the three comparison rules fire only when `expected_signers` is declared, and a key appears in exactly one finding: a declared weight of 0 states an intent to remove, so an entry the chain still shows above 0 is an unexpected signer, an entry the chain shows at 0 while the policy declares weight above 0 is missing, and only a key both sides name at different nonzero weights is drift. Third, `sig-threshold-unreachable` emits one finding per account naming every unreachable tier instead of one per tier, because a fingerprint per tier would let an operator accept one tier of a broken configuration and read the account as clean.

A high threshold of 0 is the common case rather than an edge, and it is the one `sig-single-key-high` states differently: the evidence says the tier is 0 and any single participating signature satisfies it, rather than printing "reaches 0" as if a weight had been compared. That phrasing is 6.3's pinned behavior, not a guess: the protocol drops zero-weight candidates before any comparison happens, so a match is required and no weight is.

### 6.8 `report` and `sanitize`

- Outputs: text, JSON and (P1) markdown.
- `sanitize` removes C0 and C1 control characters, whole terminal escape sequences, bidirectional override characters and zero-width characters, collapses whitespace, truncates length, and flags non-ASCII hostnames in domains (IDN lookalike risk). Markdown output additionally escapes special characters and places evidence only inside fenced blocks with a fence longer than any backtick run inside it. Implemented 2026-10-30 (task 2.5) with a 256-rune field cap, whole CSI and OSC sequence removal, Unicode tag characters, and fuzz targets for text and fencing.
- Ordering: severity descending, then account, then rule ID. Output is deterministic.
- Implemented 2026-10-30 (task 2.6): `report.Text` and `report.JSON` both render a sanitized copy, so the caller's report is never mutated. JSON writes severity as its name (`info`, `warn`, `high`) rather than the Go constant, indents by two spaces, does not escape HTML, and carries the limits footer as a `limits` field; the text report carries the same text as `limits:`. Empty collections render as `[]`, never `null`. `report.Exit` maps findings to 0 or 1, ignoring accepted findings and anything below `fail_on`; usage and source errors are decided before a report exists. Golden files live in `testdata/golden/` and are rewritten only with `-update`.

### 6.9 `run`

Orchestrates: load policy, build sources, check network passphrase, fetch snapshots concurrently with a bound, fetch identity data, evaluate rules, apply accepts, produce a `Report`, map to an exit code.

Implemented 2026-10-30 (task 2.8) as `Run(ctx, Job)` with `Check` and `Audit` as the two builders. Order is fixed: the request is validated, the source is built, the passphrase is compared **before any account read**, then accounts are fetched in policy order through a semaphore (default 8) that cancels every worker on the first failure, so a partial fetch never becomes a report. A passphrase mismatch does not stop the run: nothing is fetched, every account gets `acct-network-mismatch`, and the report says no state was read (task 3.3). A 404 is also observation rather than failure: the account becomes an `acct-missing` finding. Every other source failure still aborts with exit 3. The rule stage is an injected `Evaluator` (default `rules.Evaluate`); a panic inside it becomes a HIGH `src-rule-panic` finding and exit 3 rather than a crash, because an incomplete result must not exit 0. Accepts are applied to acceptable rules only, with the expiry date treated as the last day it holds and an unparseable date treated as not live. `report.Exit` decides 0 or 1; every orchestration failure is returned as an error, not a report, and `ErrUsage` marks the failures the CLI maps to exit 2.

Implemented 2026-11-03 (task 3.5): the order is now validate, build the source, compare the passphrase, fetch the accounts, fetch the identity documents, evaluate. `Job.NewIdentity` is the second injected factory and sits beside `Job.NewSource`; nil builds the guarded `sep1.Client`, never a skip. The stage asks only about a domain the chain actually showed, so a missing account and an account publishing no home domain produce no fetch, and the whole stage is skipped when the endpoint is on the wrong network, before any client is constructed. It runs under its own semaphore of the same `Concurrency` bound and the same cancel-on-first-failure shape as the account reads, with results keyed by index so policy order survives. An unreachable document is not an orchestration failure: it reaches the rules as a WARN, and a WARN under the CLI's default `--fail-on high` exits 0, which is the same call task 3.3 made when it turned a 404 into `acct-missing` instead of exit 3. A fetcher that will not build, or a context that ends mid-stage, is an error: no report exists and the run exits 3.

### 6.10 `cmd/stellar-gauge`

Argument parsing and dispatch only. No logic.

Implemented 2026-10-30 (task 2.9) with the standard library `flag` package, one file per command. `runCLI(args, stdout, stderr) int` is the whole entry point, so tests drive the CLI in process and there is no `os/exec` anywhere. Exit codes are `model.ExitCode` values: 0 clean, 1 findings, 2 usage or policy, 3 source or internal. `check` takes `--network`, exactly one `--endpoint`, `--format text|json`, `--fail-on high|warn` and one or more `G` accounts; `audit` takes `--config`, `--format` and `--fail-on`, refuses positional arguments, and reports `markdown` as not implemented rather than silently falling back to text. Account arguments are classified before any request: `M` and `W` get the muxed message, `C` the contract message, `S` the rotate-the-key message, and none of them is ever echoed. A rendering failure exits 3, never 0.

Implemented 2026-11-03 (task 3.5): `runCLI(args, stdout, stderr, deps)`, and `deps` carries the two factories `run` accepts. The zero value is the production value, so this package holds no copy of the guard's defaults and cannot weaken them; a test hands in an offline identity fetcher because the committed clawback fixture carries a real home domain, and letting the guarded client near it would dial a third-party host over the public internet. No new flag and no new output format came with it.

## 7. Data models

As of 2026-11-04, `internal/model` carries the severity names, the finding constructor with its fingerprint and the sort order, `internal/strkey` validates and classifies addresses, `internal/sanitize` cleans output, `internal/source` reads and strictly decodes Horizon responses, `internal/sep1` fetches a `stellar.toml` through the dial-time guard, `internal/report` renders them, `internal/rules` holds the seven account, flag and identity rules, `internal/run` orchestrates a run, and `cmd/stellar-gauge` exposes `check`, `audit` and `version`. A run can now exit 1. The two account rules are HIGH and fail the default threshold; `flag-drift`, `flag-undeclared-clawback` and the three `id-` rules are WARN, so they fail only under `--fail-on warn`. Threshold math and the signer and threshold rules are still pending. JSON tags are snake_case field names and severity marshals as its lowercase name; the report schema and serialization behavior are fixed by task 2.6 and its golden files. Signer type values (`ed25519`, `hash_x`, `pre_auth_tx`, `signed_payload`) are internal labels; `internal/source` maps them from Horizon's `ed25519_public_key`, `sha256_hash`, `preauth_tx` and `ed25519_signed_payload`.

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
| Network passphrase differs from configured network | `acct-network-mismatch` (HIGH), no state read |
| Account not found | `acct-missing` (HIGH) |
| Input is a secret seed | exit 2, value never echoed |
| Input is `M` or `C` address | exit 2 with a clear message |
| Unknown config key, missing `version`, missing `network` | exit 2 |
| `stellar.toml` unreachable, oversized, blocked by the guard or unparsable | `id-toml-unreachable` (WARN) |
| Two endpoints disagree (P1) | `src-disagreement` (HIGH) |
| Rule panics | recovered, reported as a HIGH internal finding, exit 3 |

Exit codes: 0 pass; 1 findings at or above `fail_on`; 2 configuration or usage error; 3 source or internal error. Anything nonzero fails CI.

As implemented through tasks 3.2, 3.3 and 3.5, the `acct-network-mismatch`, `acct-missing` and `id-toml-unreachable` rows are active. The two account findings are HIGH and neither can be accepted, so they fail the default threshold and keep gate G3's exit non-zero. A passphrase mismatch reads no account at all: each account is marked skipped and carries the finding. `id-toml-unreachable` is a WARN, so its fail-closed shape is different: an identity Gauge could not verify never reads as a clean report, but it stops a gate only at `--fail-on warn`, and what does abort with exit 3 is a fetcher that will not build, because then nothing was verified. `src-disagreement` remains inactive until task 5.1, and an unreachable endpoint, an oversized body or a malformed response still exits 3 with no report. Exit 1 therefore means the run completed and the chain disagreed with the request; exit 3 still means no trustworthy comparison exists.

## 9. Rule specifications

Role severity matrix: `PRD.md` Appendix A. Key definitions, all nine implemented 2026-11-04 by task 3.1 and shipping at their baseline severity because the role map is 3.4:

- **`sig-threshold-unreachable`:** for each tier (low, medium, high), if the sum of participating signer weights is below the threshold, that tier can never authorize. A high tier that is unreachable means signers and thresholds can never change again. Implemented as one finding per account naming every unreachable tier, and it is on the non-acceptable list, so no `[[accept]]` entry silences it.
- **`sig-single-key-high`:** `MinSigners(signers, high) == 1`. At the default `high` of 0 that is true for any participating key, which is the protocol's behavior rather than an artifact, so the evidence names the tier as 0 instead of printing a weight comparison that never happened.
- **`sig-hashx`:** any `HashX` signer; HIGH if its weight alone reaches the medium threshold, and WARN otherwise, including when the medium is 0 because at that tier the weight is not what does the work.
- **`sig-no-signing-weight`:** no participating signer. The account cannot sign anything. For an issuer with `expect_locked = true` this is the intended black-hole configuration and is INFO.
- **Drift rules:** compare declared signers, weights, thresholds and flags with the snapshot. Declared values are optional; undeclared fields are only reported as facts. A key appears in exactly one of `sig-unexpected-signer`, `sig-missing-expected` and `sig-weight-drift`: a declared weight of 0 is an intent to remove, so the chain showing it above 0 is an extra signer and the chain showing a nonzero declaration at 0 is a missing signer.
- **`flag-undeclared-clawback`:** clawback on and no declared expectation. Clawback must be enabled before issuance, so a later change shows up as drift. [Verified: Stellar docs]
- **Identity rules:** `id-no-home-domain`, `id-toml-unreachable` and `id-toml-account-unlisted`, implemented in task 3.5. Field names are confirmed in task 0.8. Each one reads the `*sep1.Result` the run fetched and none of them performs a fetch; `id-toml-unreachable` also fires when the account shows a home domain that was never fetched, while a `home_domain` that is blank after trimming shows none and keeps both toml rules silent, and the trigger `PRD.md` 6.1 phrases by role waits on the 3.4 role table.

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
- Airlock is intended to scan this repository in CI (roadmap 4.7). It does not today: `grep -rn airlock .github/workflows` returns nothing on `main`, and the dependency it waits on, a released Airlock, does not exist yet. Airlock's own CI runs its own scanner; that is the dogfooding that has happened.
- `scripts/check_roadmap_touched.sh` runs as a required check (section 16).

## 12. Testing strategy

- **Unit tests** for every package, with `-race`.
- **Threshold math:** exhaustive tables plus randomized comparison with a brute-force search.
- **Fake Horizon** via `httptest`: success, 404, 429 with `Retry-After`, 5xx, slow responses, oversized bodies, malformed JSON, wrong passphrase, redirects.
- **Captured fixtures** in `testdata/horizon/`: five response bodies from `https://horizon-testnet.stellar.org` on 2026-10-30 under D-16, namely three funded accounts, one 404 `problem+json` for a valid but unfunded address and the network root, with `testdata/horizon/README.md` recording the ledger each carries. Mainnet account shapes come from `posture-sec/stellar-account-fixtures` (public since 2026-11-02, nine captures since 2026-11-03, eight mainnet and one testnet): sorted-key re-serialisations of `GET /accounts/<id>` beside a sidecar holding the URL, capture time, `last_modified_ledger`, Horizon version and the SHA-256 of those bytes, which `validate.py` recomputes and refuses if the label set does not match. Nothing from that corpus is vendored here yet, and the list of shapes with no real source anywhere is two long rather than three: `hash(x)` and pre-auth signer entries, every signer key in either capture set starting with `G`, stay hand-built and labelled as such because bringing one into existence needs a transaction submission this project forbids. The claim that a master key at weight 0 cannot be found was always about scenario 2's combination, not about weight 0 itself: four reads do show the master present at weight 0 among other signers (`testdata/horizon/7-extra-signer.json` and the corpus's `coordinator-data-entry`, `sponsored-trustlines` and `testnet-multisig`), while master weight 0 with *no* other signer remains uncaptured in both sets. Task 3.6 assembles the Appendix B fixture set and is where that split gets decided with a run behind it.
- **Golden files** for text, JSON and markdown output.
- **Fuzzing:** strkey classification, config decoding, SEP-1 parsing, sanitizer. The three existing targets stay green; SEP-1 parsing has no fuzz target of its own yet, because a new one needs a roadmap task rather than an agent's invention.
- **SSRF tests:** loopback, private ranges, metadata address, DNS rebinding simulation, redirect to a private host. Implemented 2026-11-03 (task 3.5): every blocked class is fetched end to end and asserted to dial nothing at all; the rebinding simulation is a resolver that first answers a public address and then the metadata address, asserted refused on the initial hop and on a redirect hop; a dial record asserts the socket was handed a numeric address rather than the host name, which is the property the design rests on. `Proxy: nil` is pinned with an `HTTPS_PROXY` value in the environment. The two offline seams, `resolver` and `dial`, are unexported fields assignable only from in-package tests, so no caller can build a `Client` that skips the guard, and the layer-2 tests still run the real HTTP stack: an `httptest.NewTLSServer` whose certificate is installed in a pool, with the fake resolver reporting a public address for its host name, so loopback is never permitted and only the socket is relocated. What no test here can prove is a live DNS rebinding attacker, and a server that ignores the hop cap.
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

Answered in the Phase 0 pass of 2026-10-30, with the answer and where it came from. Item 2 is the one that arrived later, on 2026-11-04 and from the protocol implementation rather than from the network:

1. Does Horizon list the master key inside `signers`, and with what shape? (0.6) **Yes**, observed on testnet: one entry of `{"weight":1,"key":"<account_id>","type":"ed25519_public_key"}`. Section 6.4 records the shape. The fixtures that pin it in `testdata/` are still to be committed, so 0.6 stays open until they are.
2. Threshold edge semantics. (0.7) **Answered 2026-11-04, and not by the method the task line proposed.** The four questions (threshold 0 with a zero-weight master and no other signer, leftover signatures, unreachable tiers, `hash(x)` signing) are deterministic rules the network enforces in code, and that code is public and pinned, so they were settled by reading `stellar-core` v30.0.0 at `0e49751aa9ddf892302cca02cea7f3f5d97df9f1`, the build `testdata/horizon/root-testnet.json` names in its own `core_version` field, and re-reading the same functions at v29.0.0 `a9d72b0` instead of by submitting transactions this project forbids itself. Section 6.3 carries each answer with a file and line range, and 2.3 implements exactly that list. What the substitution does not buy: an observed account exhibiting any of these shapes, which is still the job of 3.6 and of the corpus.
3. SEP-1 fields, size limit and redirect rules. (0.8) **Answered**: `VERSION`, `NETWORK_PASSPHRASE`, `SIGNING_KEY`, `HORIZON_URL`, `ACCOUNTS`; 100 KB; no `ACCOUNT_ID`; no redirect rules in the spec, so the cap in 6.5 is Gauge's. Text in 6.5.
4. Horizon status and RPC coverage for accounts. (0.9) **Answered**: Horizon is not marked deprecated, and `getLedgerEntries` serves classic account entries. ADR-005 stands as written.
5. Own strkey versus SDK import. (0.10) **Answered**: own package. Section 6.2 carries the layout and the rejection rules the spec's test suite exercises.
6. TOML dependency footprint. (0.13) **Answered**: `BurntSushi/toml` v1.6.0, MIT, zero transitive dependencies. ADR-004.
7. Public endpoint rate limits and which endpoints to default to. (0.12) **Answered**: per-IP, 3600 per hour by default, 429 on exceed, `limit` capped at 200; testnet observed, mainnet documented only. `PRD.md` 6.4.

Still genuinely unknown, and not a Phase 0 task:

- Whether Horizon truncates the embedded `balances` array for an account with many trustlines. No captured account had more than one balance.
- The exact `signers` shape for `hash(x)`, pre-auth and `ed25519_signed_payload` entries, which cannot be funded into existence through friendbot. The three type strings are no longer unknown: each is the value the serving code puts in `type`, cited in 6.4, and `internal/source` maps all four pairs. What is still unobserved is an entry of any of the three kinds in a real response. The serialization itself is knowable from the same place, `protocol.Signer` at `protocols/horizon/main.go:361-366` of `stellar/go-stellar-sdk` v0.8.0: `weight`, `key`, `type` and an optional `sponsor`, so a signed-payload signer is a `{weight, key, type}` entry whose `key` is a `P` address that already carries the payload inside it.
- The maximum signer count is **no longer unknown**: `MAX_SIGNERS = 20` in the ledger entry and 20 signature slots in an envelope, both cited in 6.3. Still unobserved: an account that uses the cap. What the two numbers together imply, and what `Reachable` has to implement, is that a nonzero master plus a full signer list is one participating weight more than a transaction can present.

## 16. Documentation and roadmap maintenance rule

`ROADMAP.md` must be updated on every contribution (code, rules, tests, docs, CI, configuration, dependency bumps, reverts), by humans and agents alike. The same pull request must tick, untick or add the affected tasks, add a changelog entry, and refresh the progress table and `Last updated` line. Enforcement is the required `roadmap-check / gate` check running `scripts/check_roadmap_touched.sh` plus a PR template checkbox (task 1.9, done). Changes to behavior described here must also update `PRD.md`, `ARCHITECTURE_ESSENTIALS.md` and `project_structure.md` in the same PR.
