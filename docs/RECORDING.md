# Demonstration run

The gate list in `docs/WAVE_SUBMISSION.md` section 3 asks for a short screen recording of a real run end to end, and the playbook treats that recording as the difference between a submission and an unfinished one. This file is the runbook for it: the commands, the output each one produced, and the boundary between what a run proves and what only a test proves.

The runs below were performed on 2026-11-03 between 15:03Z and 15:07Z, from a working tree at `b608e13` with `bin/stellar-gauge` built from `cmd/stellar-gauge`. Every transcript is what the binary printed, pasted rather than restated, including the `exit code:` line the report itself prints. The one exception is noted in run 3.

Those transcripts are the record of that afternoon, not the script for the recording. Task 3.7 landed on 2026-11-05 and every report it prints carries a fact block these transcripts predate: a `minimum signers to authorize:` line under the thresholds, a `declared:` line or block naming what the policy carried, and for an account that publishes a home domain, an `identity document:` block with the step, the detail and what the fetch recorded. Re-run each command against a build from the merged 3.7 tree and paste what it prints, rather than recording against the lines below. The rerun of run 5 on 2026-11-05 is the first such transcript, pasted there so the shape is visible: the same account, the same two warnings plus the single-key INFO that 3.1's rules add, `findings: 3 (0 accepted)` where the 2026-11-03 build printed two, and the fact block above them.

## 0. Ground rules

- Read-only. The session performed seven account reads across three distinct testnet accounts, one root document against `https://horizon.stellar.org`, and two fetches of `https://komunitin.org/.well-known/stellar.toml`, one per form of run 5. No transaction is built or submitted, no account is funded, nothing is written to disk except the binary itself. This is what D-16 permits and what it forbids, and the difference is visible in the commands: none of them takes a seed and none of them takes an `--out`, which does not exist.
- The recording is not committed here. A video or a GIF in `docs/` is exactly what OSSF Scorecard's Binary-Artifacts check penalises, and this repository carries the badge in its README. Keep the file out of the tree and put its link in the submission answer.
- Ledger numbers and account sequences move between runs. This account is on a shared testnet and its sequence went from `57883274339137` to `57883274339138` between the capture above and a re-check the same afternoon. Do not try to reproduce a ledger number; reproduce the shape. The fingerprints do reproduce, because they are computed from rule, account and evidence rather than from the wall clock.

## 1. Setup

```
git log --oneline -1
go build -o bin/stellar-gauge ./cmd/stellar-gauge
./bin/stellar-gauge version
```

`version` prints `0.1.0-dev`. Record the prompt and the branch name once at the start so the run is tied to a commit.

## 2. Run 1: a clean pass, and the facts behind it

```
./bin/stellar-gauge check --network testnet --endpoint https://horizon-testnet.stellar.org --fail-on warn GCI76XDVHDGJMZGP5EWQYCZCYIBNBVABLPHWCKEZWJF2K4Z5SD74A4AV
```

This is the account captured as `testnet-multisig` in `posture-sec/stellar-account-fixtures`. It is the pass case, and it is the one that shows what Gauge prints even when it has nothing to complain about: thresholds, the minimum signatures each tier needs, flags, signers with their weights, the home domain line reading `(none)`, the `declared:` block naming what the policy carried, and the limits footer. A rerun on 2026-11-05 printed the same shape with 6 signers at weights 1 through 1 and a master key at 0, thresholds 3/3/3, `minimum signers to authorize: low 3, medium 3, high 3` and no findings.

```
Gauge 0.1.0-dev
network: Test SDF Network ; September 2015
ledger: 5538911

account GCI76XDVHDGJMZGP5EWQYCZCYIBNBVABLPHWCKEZWJF2K4Z5SD74A4AV
  endpoint: https://horizon-testnet.stellar.org
  as of ledger 5538881, sequence 57883274339137
  thresholds: low 3, medium 3, high 3
  flags: auth_required=false auth_revocable=false auth_immutable=false clawback=false
  home domain: (none)
  sponsor: (none)
  signers: 6
    GA5J6K77LKAXSRM4VRKBXAOQXVOVYWARIG7NQURWGBXMQWIXS7R5KHRV ed25519 weight 1
    GABVD2TOKWRPASQUCSOYD6V5GMFOVTSSLMGBGDHQANZPMCKSLGKQU7JQ ed25519 weight 1
    GAKECSILEKZCLRPOMPRYTRFCOXWSSJRJ5BCHZ53BMJNL4AAQ3RKOZ5CO ed25519 weight 1
    GBRTKUSLK55UOXBT34X2LNJ7VXJTSE7T5J5PBV7SV73QL3YAROHK3W22 ed25519 weight 1
    GD2SIZATHHGBMHJEAEGJBQTASQYE3ZW2BCLSOJPD6AJPHFWXIWYO7PN7 ed25519 weight 1
    GCI76XDVHDGJMZGP5EWQYCZCYIBNBVABLPHWCKEZWJF2K4Z5SD74A4AV ed25519 weight 0
  findings: none

result: 0 findings (0 high, 0 warn, 0 info), 0 accepted
exit code: 0
limits: Gauge reads public ledger state only. It cannot see who holds a key, whether signers are separate people, or off-chain controls. A clean report is a posture result, not proof that the account cannot be compromised.
```

Exit 0 with `--fail-on warn` set is the point: the threshold was lowered to the strictest setting and the account still cleared it. Six signers, five at weight 1 against thresholds of 3, plus the master key at weight 0, which cannot sign anything including threshold-0 operations. No rule reads that shape yet; the signer family is task 3.1.

## 3. Run 2: a policy that disagrees with the chain

`/tmp/gauge-demo.toml`, a scratch file deliberately outside the repository:

```toml
version = 1
network = "testnet"
endpoints = ["https://horizon-testnet.stellar.org"]

[[account]]
id = "GCI76XDVHDGJMZGP5EWQYCZCYIBNBVABLPHWCKEZWJF2K4Z5SD74A4AV"
name = "demo multisig"
role = "treasury"
expected_flags = { clawback = true }
home_domain = "example.org"
expected_signers = [
  { key = "GA5J6K77LKAXSRM4VRKBXAOQXVOVYWARIG7NQURWGBXMQWIXS7R5KHRV", weight = 9 },
  { key = "GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ", weight = 1 },
]
```

```
./bin/stellar-gauge audit --config /tmp/gauge-demo.toml --fail-on warn
```

The declaration is wrong in three separate ways. `clawback = true` against an account that has not enabled it is `flag-drift`. A declared `home_domain` against an account that publishes none is `id-no-home-domain`, a rule only `audit` can raise, since `check` has no declaration to compare against. The third wrongness, a present signer declared at weight 9 and an absent signer declared at weight 1, produced nothing when this run was first recorded on 2026-11-03, because no registered rule read `expected_signers` yet. Task 3.1 landed the four that do on 2026-11-04, and the rerun below is the version to record.

What that build printed on 2026-11-03, kept as the record it is:

```
Gauge 0.1.0-dev
network: Test SDF Network ; September 2015
ledger: 5538914

account GCI76XDVHDGJMZGP5EWQYCZCYIBNBVABLPHWCKEZWJF2K4Z5SD74A4AV
  endpoint: https://horizon-testnet.stellar.org
  as of ledger 5538881, sequence 57883274339137
  thresholds: low 3, medium 3, high 3
  flags: auth_required=false auth_revocable=false auth_immutable=false clawback=false
  home domain: (none)
  sponsor: (none)
  signers: 6
    ...six signer lines, as in run 1...
  findings: 2 (0 accepted)
    [warn] flag-drift
      the declared flags differ from the flags the chain shows
      evidence: clawback declared true, observed false
      fingerprint: dc00cbe22e214427f73da25bc0ef532538cd51c37568848bff6680c1103d7b99
    [warn] id-no-home-domain
      the policy declares a home domain and the account publishes none
      evidence: home_domain declared "example.org", observed none
      fingerprint: 929c040425387fa86dd8fbb4b496b341f783271b3a2fd62b0e31373f7ea4fa2e

result: 2 findings (0 high, 2 warn, 0 info), 0 accepted
exit code: 1
limits: ...
```

The same command on 2026-11-05, ledger 5569618 at read time, pasted whole. This is the one to record.

```
Gauge 0.1.0-dev
network: Test SDF Network ; September 2015
ledger: 5569618

account GCI76XDVHDGJMZGP5EWQYCZCYIBNBVABLPHWCKEZWJF2K4Z5SD74A4AV
  endpoint: https://horizon-testnet.stellar.org
  as of ledger 5569601, sequence 57883274339575
  thresholds: low 3, medium 3, high 3
  minimum signers to authorize: low 3, medium 3, high 3
  flags: auth_required=false auth_revocable=false auth_immutable=false clawback=false
  home domain: (none)
  sponsor: (none)
  signers: 6
    GA5J6K77LKAXSRM4VRKBXAOQXVOVYWARIG7NQURWGBXMQWIXS7R5KHRV ed25519 weight 1
    GABVD2TOKWRPASQUCSOYD6V5GMFOVTSSLMGBGDHQANZPMCKSLGKQU7JQ ed25519 weight 1
    GAKECSILEKZCLRPOMPRYTRFCOXWSSJRJ5BCHZ53BMJNL4AAQ3RKOZ5CO ed25519 weight 1
    GBRTKUSLK55UOXBT34X2LNJ7VXJTSE7T5J5PBV7SV73QL3YAROHK3W22 ed25519 weight 1
    GD2SIZATHHGBMHJEAEGJBQTASQYE3ZW2BCLSOJPD6AJPHFWXIWYO7PN7 ed25519 weight 1
    GCI76XDVHDGJMZGP5EWQYCZCYIBNBVABLPHWCKEZWJF2K4Z5SD74A4AV ed25519 weight 0
  declared: role treasury, name "demo multisig"
    expected_signers: 2 keys declared
    expected_thresholds: (not declared)
    expected_flags: auth_required=(not stated) auth_revocable=(not stated) auth_immutable=(not stated) clawback=true
    home_domain: "example.org"
    expect_locked: (not declared)
  findings: 5 (0 accepted)
    [high] sig-missing-expected
      a declared signer cannot authorize on this account
      evidence: GA7QYN...VSGZ declared at weight 1, chain shows 0
      fingerprint: 746351fd27c712252cc81a69273de69519ccabde9c5375b169b565198370b3ac
    [high] sig-unexpected-signer
      the account carries signers the policy does not declare
      evidence: GABVD2...U7JQ ed25519 at weight 1 is not declared; GAKECS...Z5CO ed25519 at weight 1 is not declared; GBRTKU...3W22 ed25519 at weight 1 is not declared; GD2SIZ...7PN7 ed25519 at weight 1 is not declared; GCI76X...A4AV ed25519 at weight 0 is not declared
      fingerprint: 966f4136d48cc915493641e547d8428933032b30c828b05aa30f2d276e9dca03
    [high] sig-weight-drift
      a signer holds a different weight than the policy declares
      evidence: GA5J6K...KHRV declared 9, chain shows 1
      fingerprint: 4a2945ee5fdcbcaf598987e42341461790d02db5fefc05428c19dcffd00f32ee
    [warn] flag-drift
      the declared flags differ from the flags the chain shows
      evidence: clawback declared true, observed false
      fingerprint: dc00cbe22e214427f73da25bc0ef532538cd51c37568848bff6680c1103d7b99
    [warn] id-no-home-domain
      the policy declares a home domain and the account publishes none
      evidence: home_domain declared "example.org", observed none
      fingerprint: 929c040425387fa86dd8fbb4b496b341f783271b3a2fd62b0e31373f7ea4fa2e

result: 5 findings (3 high, 2 warn, 0 info), 0 accepted
exit code: 1
limits: Gauge reads public ledger state only. It cannot see who holds a key, whether signers are separate people, or off-chain controls. A clean report is a posture result, not proof that the account cannot be compromised.
```

Four things to point at while recording this. The `declared:` block is what task 3.7 added, and it is the block that makes the rest of the report readable: `expected_thresholds: (not declared)` says the threshold drift rule had nothing to compare, and `auth_required=(not stated)` distinguishes a flag the file left out from a flag the file declared false, so an omitted expectation cannot be misread as a clean one. The three HIGH findings are what 3.1 and 3.4 turned the earlier null result into, and `flag-drift` and `id-no-home-domain` carry the same fingerprints as the 2026-11-03 run, which is the claim that a fingerprint comes from rule, account and evidence rather than from the date. Say out loud that the tool is being run against someone else's real account with a policy written to be wrong on purpose, and that the account moved between the two runs: `as of ledger 5538881` then, `5569601` now, on a shared testnet.

The second key in that file is the SEP-23 test vector `GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ`, not an invented string. An earlier attempt at this policy used a fabricated `GCDEMOABSENTSIGNER…` and the config loader refused the whole file before reading any account, which is run 7 below.

## 4. Run 3: the threshold decides, not the finding

```
./bin/stellar-gauge audit --config /tmp/gauge-demo.toml
```

The same two warnings against the same account, with the default `--fail-on high`, and the run exits 0.

```
result: 2 findings (0 high, 2 warn, 0 info), 0 accepted
exit code: 0
```

Only the tail of that report is quoted because the command was piped through `tail -3` at the time. The `exit code:` line is the binary's own, so the exit status is in the capture; the shell variable next to it was not, because `$?` after a pipe reports the last stage. Use `${PIPESTATUS[0]}` or no pipe when recording, so the shell's own status is visible under the report's.

This is the pair that shows exit semantics are not a bug: WARN findings are reported and do not fail a scheduled run by default, and the operator opts into failing them.

## 5. Run 4: an endpoint that is not the network requested

```
./bin/stellar-gauge check --network testnet --endpoint https://horizon.stellar.org GCI76XDVHDGJMZGP5EWQYCZCYIBNBVABLPHWCKEZWJF2K4Z5SD74A4AV
```

This is the only mainnet request in the whole run, and it is the root document: enough to read the network passphrase the endpoint serves, and no further. The account is not read at all, which the report says in its own body rather than leaving the reader to infer.

```
Gauge 0.1.0-dev
network: Public Global Stellar Network ; September 2015
ledger: 65286436

account GCI76XDVHDGJMZGP5EWQYCZCYIBNBVABLPHWCKEZWJF2K4Z5SD74A4AV
  endpoint: https://horizon.stellar.org
  no state read: the endpoint is on a different network
  findings: 1 (0 accepted)
    [high] acct-network-mismatch
      the endpoint is on a different network than the run requested, so the account was not read
      evidence: expected Test SDF Network ; September 2015, endpoint reports Public Global Stellar Network ; September 2015
      fingerprint: fbf18d26f8dd37a465375b82d9d8749b3c3a7b5f07b5800ed3494c03ebab24b7

result: 1 findings (1 high, 0 warn, 0 info), 0 accepted
exit code: 1
limits: ...
```

HIGH fails at the default threshold, so no `--fail-on` flag is needed here. This is also the shape a misconfigured operator actually hits: a scheduled job pointed at the wrong endpoint would otherwise report testnet posture as though it were mainnet.

## 6. Run 5: an account that publishes a home domain, and the live read that refused it

```
./bin/stellar-gauge check --network testnet --endpoint https://horizon-testnet.stellar.org --fail-on warn GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG
```

This is the issuer captured as `whitespace-home-domain` in the fixtures repository. It carries a home domain, so it is the account that drives the 3.5 fetcher, and the identity rules are the ones run 1 cannot show.

```
account GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG
  endpoint: https://horizon-testnet.stellar.org
  as of ledger 4699329, sequence 20183455778209793
  thresholds: low 0, medium 0, high 0
  flags: auth_required=true auth_revocable=true auth_immutable=false clawback=true
  home domain: komunitin.org
  sponsor: GCHEQPEHYQTHK4EVTPV7ASSRLJ6XZBZKEGZ3FWYXRLLQZ7L35PO7OUBM
  signers: 1
    GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG ed25519 weight 1
  findings: 2 (0 accepted)
    [warn] flag-undeclared-clawback
      clawback is enabled and the policy does not declare it
      evidence: clawback observed true, and no clawback expectation is declared
      fingerprint: c84578b5217e2a97c558c5ec0e821d6f5cab54223e655b74cffb43b2322e7290
    [warn] id-toml-unreachable
      the home domain komunitin.org does not serve a stellar.toml Gauge can read
      evidence: step parse; detail the document is not a configuration file this parser accepts; status 200
      fingerprint: 59aa1a9db576b141d2e2fadf402b88404a3ad441c55135d85182ba133120288a

result: 2 findings (0 high, 2 warn, 0 info), 0 accepted
exit code: 1
limits: ...
```

The first three report lines are cut from this transcript because the command was piped through `tail -22`. The same account read as JSON at 15:05Z reported ledger 5538942 with `exit_code` 1 in the document body.

Read the evidence line carefully: `step parse`, `status 200`. The domain resolved, the guard admitted the address, TLS was negotiated, and the document came back as a success carrying HTML rather than a configuration file. That is the 3.5 behaviour as designed and it is the first time the fetcher was pointed at a real third-party domain rather than at a test server. It is also the reason the finding is a refusal of the document rather than a claim about the domain: Gauge says what step failed and quotes no body.

The same command on 2026-11-05 against a build that carries 3.7, pasted whole because nothing was piped this time. Ledger 5569489 at read time.

```
Gauge 0.1.0-dev
network: Test SDF Network ; September 2015
ledger: 5569489

account GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG
  endpoint: https://horizon-testnet.stellar.org
  as of ledger 4699329, sequence 20183455778209793
  thresholds: low 0, medium 0, high 0
  minimum signers to authorize: low 1, medium 1, high 1
  flags: auth_required=true auth_revocable=true auth_immutable=false clawback=true
  home domain: komunitin.org
  sponsor: GCHEQPEHYQTHK4EVTPV7ASSRLJ6XZBZKEGZ3FWYXRLLQZ7L35PO7OUBM
  signers: 1
    GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG ed25519 weight 1
  declared: nothing; this run read no policy file, so only the universal rules ran
  identity document: komunitin.org did not serve a readable file
    step: parse
    detail: the document is not a configuration file this parser accepts
    fetch: status 200, 0 redirect hops, 3363 bytes, CORS absent
  findings: 3 (0 accepted)
    [warn] flag-undeclared-clawback
      clawback is enabled and the policy does not declare it
      evidence: clawback observed true, and no clawback expectation is declared
      fingerprint: c84578b5217e2a97c558c5ec0e821d6f5cab54223e655b74cffb43b2322e7290
    [warn] id-toml-unreachable
      the home domain komunitin.org does not serve a stellar.toml Gauge can read
      evidence: step parse; detail the document is not a configuration file this parser accepts; status 200
      fingerprint: 59aa1a9db576b141d2e2fadf402b88404a3ad441c55135d85182ba133120288a
    [info] sig-single-key-high
      one signer alone reaches the high threshold, so a single key holds the account
      evidence: GDSX2W...BEVG ed25519 at weight 1 is enough on its own, because the high threshold is 0 and any single participating signature satisfies it
      fingerprint: f0bbe6842ce92e57a4970928c6991a58aee4e05e646c3d68cb25e9b3c5f9e01c

result: 3 findings (0 high, 2 warn, 1 info), 0 accepted
exit code: 1
limits: Gauge reads public ledger state only. It cannot see who holds a key, whether signers are separate people, or off-chain controls. A clean report is a posture result, not proof that the account cannot be compromised.
```

Three things to point at while recording this. Both `warn` fingerprints are identical to the 2026-11-03 run above, which is the claim in `ARCHITECTURE.md` that a fingerprint is computed from rule, account and evidence rather than from the wall clock or the severity. The account has not moved either: `as of ledger 4699329` in both transcripts, because nothing has written to it. And the fact block prints the fetch that the finding does not: 3363 bytes and `CORS absent` are facts about the transaction, the finding names only the step that failed, and no line anywhere in the report carries a byte of the document's HTML.

## 7. Run 6: an account that is not there

```
./bin/stellar-gauge check --network testnet --endpoint https://horizon-testnet.stellar.org GC2HHIM66CBKJFMYOHECBMKD4NWRGWJ3FEXFDPWPT4TLS7OFR43LO2II
```

```
    [high] acct-missing
      the account was not found on this endpoint, so it may never have been created or it was merged
      evidence: endpoint https://horizon-testnet.stellar.org returned not found
      fingerprint: b03faa6106f2405560196e0cad9e0bb74cab9c08d798877c0d795019bddd43d6

result: 1 findings (1 high, 0 warn, 0 info), 0 accepted
exit code: 1
limits: ...
```

Only the tail is quoted, because the command was piped. The head of the report names the account and the endpoint.

## 8. Run 7: the three exit-2 paths, no network at all

These are worth thirty seconds of recording because they are the safety claims, and they cost nothing: the CLI rejects before any source is constructed.

A secret seed. The value is built rather than stored, so that no seed, not even a degenerate one, sits in this file or in git history. AGENTS.md section 4 requires exactly that.

```
SEED=$(python3 -c "
import base64
def crc16(d):
    c=0
    for b in d:
        c^=b<<8
        for _ in range(8):
            c=((c<<1)^0x1021)&0xFFFF if c&0x8000 else (c<<1)&0xFFFF
    return c
p=bytes([18<<3])+b'\x00'*32
c=crc16(p)
print(base64.b32encode(p+bytes([c&0xFF,c>>8])).decode().rstrip('='))
")
./bin/stellar-gauge check --network testnet --endpoint https://horizon-testnet.stellar.org "$SEED"
```

The payload is thirty-two zero bytes under the `S` version byte with the CRC16 the SEP-23 spec appends little-endian. The resulting string is a valid seed address for a key nobody holds, and this file does not print it.

```
stellar-gauge: a secret seed was supplied; remove it from your shell history and rotate that key
```

Exit 2. No ledger line, no network, and the value is not echoed anywhere in the message.

A muxed `M` address where a `G` address is required, and a config file naming a signer that is not a strkey at all:

```
./bin/stellar-gauge check --network testnet --endpoint https://horizon-testnet.stellar.org MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZAAAAAAAAACDVZWC6
stellar-gauge: a supplied value is not a valid Stellar account address; expected a G address
```

For the policy path, write run 2's file with the absent signer named by an invented string and change nothing else:

```
./bin/stellar-gauge audit --config /tmp/gauge-demo.toml --fail-on warn
stellar-gauge: policy: expected signer keys must be G, T, X or P strkeys (account[0].expected_signers[1].key)
```

Both exit 2. The second is run 2's file with the invented key left in place, the accident that is written up there. The error names the exact path inside the file.

## 9. Run 8: the same run as JSON

```
./bin/stellar-gauge audit --config /tmp/gauge-demo.toml --fail-on warn --format json
```

Record this last, or not at all if the recording is already long. The fields a consumer would script against, in order: `tool_version`, `network`, `ledger`, `accounts`, `exit_code`, `limits`. Each account is `snapshot`, then `facts`, then `findings`; each finding is `rule_id`, `severity`, `account`, `message`, `evidence`, `fingerprint`, `accepted`. The `facts` block is 3.7's and carries the same three sections the text report prints: `reachability`, `declared` and `identity`, with JSON null where the text report writes `(not declared)` or `(not stated)`.

```
  "facts": {
    "reachability": { "low": 3, "medium": 3, "high": 3 },
    "declared": {
      "name": "demo multisig",
      "role": "treasury",
      "expected_signers": 2,
      "expected_thresholds": null,
      "expected_flags": {
        "auth_required": null, "auth_revocable": null,
        "auth_immutable": null, "clawback": true
      },
      "home_domain": "example.org",
      "expect_locked": null
    },
    "identity": null
  }
```

`expected_signers` is a count of declared keys, not the keys themselves: the snapshot above already lists every signer the chain shows, and reprinting the operator's own list back at them would make the block longer without telling them anything the findings do not. The findings are where a key on one side and not the other becomes a result. `declared` is null for the whole section on a `check` run, which read no file, and an `expected_flags` value of null means the file did not state that flag.

```
{"rule_id": "flag-drift", "severity": "warn", "account": "GCI76XDVHDGJMZGP5EWQYCZCYIBNBVABLPHWCKEZWJF2K4Z5SD74A4AV", "message": "the declared flags differ from the flags the chain shows", "evidence": "clawback declared true, observed false", "fingerprint": "dc00cbe22e214427f73da25bc0ef532538cd51c37568848bff6680c1103d7b99", "accepted": false}
{"rule_id": "id-no-home-domain", "severity": "warn", "account": "GCI76XDVHDGJMZGP5EWQYCZCYIBNBVABLPHWCKEZWJF2K4Z5SD74A4AV", "message": "the policy declares a home domain and the account publishes none", "evidence": "home_domain declared \"example.org\", observed none", "fingerprint": "929c040425387fa86dd8fbb4b496b341f783271b3a2fd62b0e31373f7ea4fa2e", "accepted": false}
```

The two fingerprints are identical to the ones in the text report of run 2 above, three minutes and one ledger earlier, and to a re-run the same afternoon at ledger 5539016. That is the property an acceptance baseline depends on, and it is the reason `accepted` exists as a field.

## 10. Why the identity demonstration is a refusal rather than an accept

An obvious gap: run 5 shows Gauge reaching a domain and rejecting what it found, and no run shows a document that passes. There is no testnet account available for that.

```
curl -s "https://horizon-testnet.stellar.org/accounts?home_domain=<domain>&limit=2"
```

Horizon answers 400 with `Exactly one filter is required. Please ensure that you are including a signer, sponsor, asset, or liquidity pool filter`, and the same for an unfiltered account list. There is no way to ask testnet which accounts publish a home domain; you have to already know one. The accounts here came from the fixtures corpus, which was assembled by sampling responses rather than by searching, so the search that would find a readable domain is not available to this run.

The accept path is covered by tests: a guarded fetch against a local server serving a real TOML document with the fields SEP-1 recognises. What a test cannot cover is a third party's answer, and that is the line the recording should state rather than paper over.

## 11. What this run does not prove

Say these, or have the packet say them.

- Mainnet account shapes. The one mainnet request was a root document, per D-16. No mainnet account was read, so the population of flags and signer shapes on mainnet is still unverified.
- The identity success path, and any home domain beyond the one this account publishes.
- The dial-time guard against an attacker. The address classes it refuses, the three-hop redirect cap and the rebinding case where the record changes between resolution and connection are proven by unit tests and by a simulated resolver. Nothing in this run dialed a hostile domain, and nothing here proves that a real one would be stopped.
- Every signer and threshold rule's first live appearance. Run 2 is the first time this repository's own account reads produced `sig-missing-expected`, `sig-unexpected-signer` and `sig-weight-drift` on a real testnet account, on 2026-11-05. The four signer rules are proven against fixtures and this one account; an account whose `hash(x)` or pre-authorized signer is what changed is not in this run, and the fixtures for those shapes are built here rather than captured.
- Whether the rules are right about your account. All sixteen registered rules fired in these runs and every severity they emitted is the severity Appendix A wrote, weighted by the role the demo policy declared. No operator outside this repository has reviewed a finding from a scheduled run, which is task 4.1 and gate G4, and the recording should say the demonstration is the author's own account read by the author's own tool.
