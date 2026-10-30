# Horizon fixtures

Captured for ROADMAP task 0.6 from `https://horizon-testnet.stellar.org` on 2026-10-30 (UTC), under decision D-16, which limits verification runs to testnet. Each file is the response body exactly as served, saved byte for byte; the account data is public ledger state.

| File | Appendix B scenario | Subject | Ledger in the response |
| --- | --- | --- | --- |
| `root-testnet.json` | none (the network root) | the endpoint itself | `history_latest_ledger` 5466897 |
| `1-default.json` | 1, and the missing-home-domain half of 9 | `GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3` | `last_modified_ledger` 5465023 |
| `7-extra-signer.json` | 7, and the multisig case task 0.6 names | `GBQHXYDEJD6NEIUQ5NYDBVNTRCACJXMLO32JUQHBGO524NMVYOMIN5CP` | `last_modified_ledger` 5466881 |
| `8-clawback-undeclared.json` | 8, issuer flags and clawback | `GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG` | `last_modified_ledger` 4699329 |
| `10-not-found.json` | 10, the not-found half | `GC2HHIM66CBKJFMYOHECBMKD4NWRGWJ3FEXFDPWPT4TLS7OFR43LO2II` | `Latest-Ledger` header 5466896 |

## What each capture establishes

`root-testnet.json`: the keys the network check reads (`network_passphrase` = `Test SDF Network ; September 2015`) and the ledger fields available for the "as of" line (`history_latest_ledger`, `ingest_latest_ledger`, `core_latest_ledger`), plus `horizon_version` and `current_protocol_version`.

`1-default.json`: the shape of a freshly funded account. Thresholds are all 0, `data` is an empty object rather than absent, `balances` holds one native entry with decimal strings, and **the master key is listed inside `signers`** as `{"weight":1,"key":"<account_id>","type":"ed25519_public_key"}`. There is no `home_domain` key at all, which is the "home domain missing" input for scenario 9.

`7-extra-signer.json`: a real multisig account, six signers and thresholds 3/3/3 with five subentries. A third party operates it; it is captured as-is because its shape, not its owner, is the point. It is the raw material for `sig-single-key-high`, `MinSigners` and `sig-unexpected-signer`.

`8-clawback-undeclared.json`: an issuer with `auth_required`, `auth_revocable` and `auth_clawback_enabled` true, one signer, weight 1, thresholds 0, and `home_domain` `komunitin.org`. That domain answered `/.well-known/stellar.toml` with HTTP 200, `content-type: text/html` and an HTML single-page-application body, which is why the SEP-1 check keys on a TOML parse rather than a content type (`ARCHITECTURE.md` 6.5). The HTML is not committed here; the URL and the observation are.

`10-not-found.json`: a `404` `application/problem+json` body for a well-formed `G` address that was never funded. The address was generated for this fixture by encoding a fixed SHA-256 digest as a strkey; it is not a secret and holds nothing.

## Not captured, and why

These Appendix B scenarios cannot be produced without submitting a transaction, which this project forbids itself, so they are hand-built in the tests that need them rather than captured:

- 2, master weight 0 and no other signers.
- 5, a `hash(x)` signer.
- 6, a pre-auth transaction signer.
- The leftover-signature case behind task 0.7.

Two more are about input handling rather than chain state and belong in tests: a mismatched passphrase (a fake Horizon returns another network's passphrase) and muxed, contract or seed strings as input.

## Perishability

Testnet resets, so an account in this directory can cease to exist while the file still parses. Nothing here is fetched at test time; the files are the record. `root-testnet.json` is also the only fixture whose `horizon_version` will drift as the testnet deployment is upgraded.
