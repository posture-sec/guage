# Horizon fixtures

Two kinds of file live here.

Captured: response bodies from `https://horizon-testnet.stellar.org`, saved byte
for byte on 2026-10-30 (UTC) under decision D-16, which limits verification runs
to testnet. The account data in them is public ledger state.

Built: account JSON this repository wrote on 2026-11-05 for ROADMAP task 3.6,
because the shape an Appendix B scenario needs cannot be produced without
submitting a transaction, which this project forbids itself. No such account
exists on any network and nothing here is fetched at test time.

| File | Kind | Appendix B scenario | Subject |
| --- | --- | --- | --- |
| `root-testnet.json` | captured | none (the network root) | the endpoint itself, `history_latest_ledger` 5466897 |
| `1-default.json` | captured | 1, and both missing-home-domain halves of 9 | `GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3` at `last_modified_ledger` 5465023 |
| `2-master-zero.json` | built | 2, and the locked-issuer variant 2b | `GBIY5RL644R6YRLYDZD5Y6VZXOWR3ZO2MTK4TSVTD4MAORSBKMKKO4RP` at 5466901 |
| `3-unreachable-high.json` | built | 3 | `GALGVY52QMWYIRPFP3EBCQZNQZXW4BPG4WWJAQ4KICLFGZH6VLSL6VMI` at 5466902 |
| `4-thresholds-out-of-order.json` | built | 4 | `GAGCZPLXXENDIGTXUJC4MG7N3NVW5TA37PFEBDQK64KREAGD2HM2LD6C` at 5466903 |
| `5-hashx-signer-at-medium.json` | built | 5 | `GBDSJWRDC7Q4QRQ5L3FTX42CRDXEGT5CJRL4ZREVTO3HKIJCM3A3UKDD` at 5466904 |
| `6-preauth-signer-only.json` | built | 6 | `GBNLMMJ6ZMKSID5OZBZD7B3IPE2WHWOKOETFBZ5M4V563YQRX53CUDWR` at 5466905 |
| `7-extra-signer.json` | captured | 7, and the multisig case task 0.6 names | `GBQHXYDEJD6NEIUQ5NYDBVNTRCACJXMLO32JUQHBGO524NMVYOMIN5CP` at 5466881 |
| `8-clawback-undeclared.json` | captured | 8 | `GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG` at 4699329 |
| `9-home-domain-present.json` | built | the reachable-domain halves of 9 | `GBKDXQ7LCO3X23K4HUKSHCA3KMM3364M4UK3526XYMBWTDD4NJCSRNAI` at 5466906 |
| `10-not-found.json` | captured | 10, the not-found half | `GC2HHIM66CBKJFMYOHECBMKD4NWRGWJ3FEXFDPWPT4TLS7OFR43LO2II`, a `404` with `Latest-Ledger` 5466896 |
| `12-hostile-home-domain.json` | built | 12 | `GDF6B4EQSNXLTSN4QROIVYLIKDFGUOZLCAGGWSGFAKPY7US72Y4A2IRJ` at 5466907 |

Scenario 10 has a second half, the network passphrase mismatch, and it needs no
account file: the run asks for a mainnet network against a testnet endpoint, so
nothing is read and the fixture under test is `root-testnet.json`. Scenario 11 is
input handling rather than chain state and has no fixture either.

## What each capture establishes

`root-testnet.json`: the keys the network check reads (`network_passphrase` =
`Test SDF Network ; September 2015`) and the ledger fields available for the "as
of" line (`history_latest_ledger`, `ingest_latest_ledger`, `core_latest_ledger`),
plus `horizon_version` and `current_protocol_version`.

`1-default.json`: the shape of a freshly funded account. Thresholds are all 0,
`data` is an empty object rather than absent, `balances` holds one native entry
with decimal strings, and the master key is listed inside `signers` as
`{"weight":1,"key":"<account_id>","type":"ed25519_public_key"}`. There is no
`home_domain` key at all, which is the "home domain missing" input for scenario 9
and for the two role-and-declaration cases that read it.

`7-extra-signer.json`: a real multisig account, six signers and thresholds 3/3/3
with five subentries. A third party operates it; it is captured as-is because its
shape, not its owner, is the point. It is the raw material for
`sig-single-key-high`, `MinSigners` and `sig-unexpected-signer`.

`8-clawback-undeclared.json`: an issuer with `auth_required`, `auth_revocable`
and `auth_clawback_enabled` true, one signer, weight 1, thresholds 0, and
`home_domain` `komunitin.org`. That domain answered `/.well-known/stellar.toml`
with HTTP 200, `content-type: text/html` and an HTML single-page-application
body, which is why the SEP-1 check keys on a TOML parse rather than a content
type (`ARCHITECTURE.md` 6.5). The HTML is not committed here; the URL and the
observation are.

`10-not-found.json`: a `404` `application/problem+json` body for a well-formed
`G` address that was never funded. The address was generated for this fixture by
encoding a fixed SHA-256 digest as a strkey; it is not a secret and holds
nothing.

## How the built files were written

The shape is the capture's, not an invention: field order, key names, the
`_links` block, `data` as an empty object and a single native `balances` entry
are copied from `1-default.json`, so a fixture exercises the decoder that a real
response exercises. `internal/source` decodes `thresholds` and `flags` strictly
and requires a signer's Horizon type string to agree with its strkey version
byte, so a built file cannot get away with a shape the endpoint does not serve.
`subentry_count` is `len(signers) - 1`, because the master key is not a
subentry.

Each address is `strkey(sha256("gauge fixture: <label>"))`, encoded with the
project's own scheme: version byte `G`, 32-byte payload, CRC16-XMODEM checksum
little-endian, RFC 4648 base32 with no padding. The `sha256_hash`, `preauth_tx`
and `ed25519_signed_payload` keys are the same construction at version bytes `X`
and `T`. Nothing was picked by hand, so nothing here is a secret or an account
anyone holds.

Two details are load-bearing:

`2-master-zero.json` is also the locked-issuer case (2b). The bytes are
identical; only the policy entry changes, and that is the point: an issuer that
declares `expect_locked` means this black hole and gets INFO for it.

`12-hostile-home-domain.json` holds the hostile field as the chain would serve
it: `home_domain` is `"\u001b[31mgauge\u202e.example.invalid"`, an ANSI colour
sequence and a right-to-left override, written as JSON `\u` escapes so the bytes
survive a checkout and a patch. The decoder hands that string over untouched and
`internal/sep1` refuses it at its hostname step; `sanitize` alters it only at the
render. `internal/run/scenarios_test.go` asserts all three of those statements
rather than trusting the golden that prints the clean-looking result.

## Not captured, and why

These shapes stay in tests, where a valid strkey is assembled at test time
(`internal/source/horizon_test.go`) rather than committed as a response:

- An `ed25519_signed_payload` signer, which task 2.11 added to the decoder.
- The leftover-signature case behind task 0.7.
- Muxed `M...`, contract `C...` and secret-seed strings as input: scenario 11,
  refused by argument parsing before any fetch.

## Perishability

Testnet resets, so a captured account in this directory can cease to exist while
the file still parses. Nothing here is fetched at test time; the files are the
record. `root-testnet.json` is also the only fixture whose `horizon_version` will
drift as the testnet deployment is upgraded.
