<p align="center">
  <img src="docs/banner.svg" alt="Gauge: read-only posture audits for Stellar accounts" width="100%" />
</p>

<p align="center">
  <a href="https://github.com/posture-sec/guage/actions/workflows/ci.yml"><img src="https://github.com/posture-sec/guage/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/posture-sec/guage/actions/workflows/roadmap-check.yml"><img src="https://github.com/posture-sec/guage/actions/workflows/roadmap-check.yml/badge.svg" alt="Roadmap check" /></a>
  <img src="https://img.shields.io/badge/go-1.27.3-00ADD8" alt="Go 1.27.3" />
  <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Apache-2.0" />
  <a href="SECURITY.md#status-of-this-code"><img src="https://img.shields.io/badge/status-unaudited-orange" alt="Unaudited" /></a>
  <a href="ROADMAP.md"><img src="https://img.shields.io/badge/status-scaffold%20only-lightgrey" alt="Scaffold only" /></a>
</p>

# Gauge

**Working name Gauge, binary `stellar-gauge`.** It reads the public configuration of your Stellar accounts and compares it with what you said you expect. Signers, thresholds, flags, home domain. It reports the difference and exits nonzero when the difference matters, so a scheduled run in CI catches a change to a treasury or issuer account the day it happens instead of the day it costs something.

The problem it addresses: an account's control settings live on-chain, are public, and almost nobody reviews them. A signer added during an incident stays forever. A threshold that no longer matches the key layout makes a "2-of-3" account 1-of-1. A clawback flag appears without anyone declaring it. These are configuration facts, so they are checkable without touching a private key.

Gauge is read-only and secret-free. It never accepts, stores or echoes a secret seed, never builds or submits a transaction, and treats every value it fetches as hostile input.

## Status

Scaffold only (ROADMAP task 1.12, decision D-09). The module, package boundaries, data models and a non-operational CLI exist:

```bash
go run ./cmd/stellar-gauge   # prints a scaffold notice and exits 3
```

No subcommand audits an account yet, so there is no release to install and no report to show. The commands below describe the intended interface and are marked as such in `README` until they work.

## What it will check

Sixteen rules in v0.1, each with a stable kebab-case ID. Severity depends on the account's declared role (issuer and treasury are held to a tighter bar than merchant and distribution).

| Family | Rules |
| --- | --- |
| Account | `acct-missing`, `acct-network-mismatch` |
| Signers | `sig-no-signing-weight`, `sig-threshold-unreachable`, `sig-threshold-order`, `sig-single-key-high`, `sig-hashx`, `sig-unexpected-signer`, `sig-missing-expected`, `sig-weight-drift` |
| Thresholds and flags | `thr-drift`, `flag-drift`, `flag-undeclared-clawback` |
| Identity | `id-no-home-domain`, `id-toml-unreachable`, `id-toml-account-unlisted` |

Exit codes: `0` pass, `1` findings at or above `--fail-on`, `2` configuration or usage error, `3` source or internal error. Anything unreachable, oversized, malformed or ambiguous fails closed. It never exits 0 on a check it could not complete.

## What this does not check

- Key custody. On-chain data cannot show who holds a key, how it is stored, or whether it leaked.
- Whether an account is a good account. Gauge reports posture against your declared expectations. It never says an account is secure or safe, and "no findings" is not a clean bill of health.
- Transactions, balances, offers, claimable balances, liquidity pools and Soroban contract accounts.
- Live streaming or alerting. It is a state comparison you schedule, not a daemon.
- The expectations themselves. If an attacker can edit both `gauge.toml` and the workflow that runs Gauge, the check can be made to pass. That residual risk is documented in `ARCHITECTURE_ESSENTIALS.md` section 10 and is a process problem, not a code problem.

## Planned usage

Target interface, not yet implemented. Once task 2.9 lands:

```bash
stellar-gauge check G... G... --network mainnet          # facts plus universal rules, no config
stellar-gauge audit --config gauge.toml --format json    # declared expectations, CI-friendly
stellar-gauge version
```

`gauge.example.toml` shows the policy shape: one `[[account]]` block per audited account with its role and expected signers, thresholds and flags, plus `[[accept]]` entries for risks you have decided to live with. An omitted expectation is not a zero or a false; it means "not declared", and drift rules only fire where you declared something.

## How it works

```
policy loader + Source + sep1 -> snapshots -> rules -> accept filter -> report -> exit code
```

| Package | Responsibility |
| --- | --- |
| `internal/model` | Shared types: snapshots, findings, reports, severities |
| `internal/strkey` | Address classification and secret-seed rejection |
| `internal/threshold` | Authorization math: reachability and minimum signers |
| `internal/sanitize` | Strips control, bidi and zero-width characters from chain data |
| `internal/policy` | Strict `gauge.toml` decode, roles, accepted risks |
| `internal/source` | Horizon client behind a `Source` interface |
| `internal/sep1` | Guarded `stellar.toml` fetch for identity rules |
| `internal/rules` | Pure rule functions, no I/O |
| `internal/report` | Text and JSON renderers, exit-code mapping |
| `internal/run` | Orchestration |

Imports point one way: `model <- strkey, threshold, sanitize <- policy, source, sep1 <- rules <- report <- run <- cmd`. Dependencies: Go standard library plus one TOML parser. No framework, no plugin system, no rule language.

## Documentation

- `PRD.md`: scope, users, requirements
- `ARCHITECTURE.md`: design, data models, decision records, red-team review
- `ARCHITECTURE_ESSENTIALS.md`: the hard rules an agent must not break
- `ROADMAP.md`: every remaining task, progress and changelog
- `project_structure.md`: where files go and who may change them
- `AGENTS.md`: working agreement for coding agents

## Maintainers

| Maintainer | GitHub | Contact |
| --- | --- | --- |
| Ademola | [@bolabeatz](https://github.com/bolabeatz) | [GitHub Discussions](https://github.com/posture-sec/guage/discussions) |

## Contributing

Read `CONTRIBUTING.md`. Every pull request updates `ROADMAP.md` in the same change, and every rule ships with a positive and a negative fixture.

<p align="center">
  <a href="https://github.com/posture-sec/guage/graphs/contributors">
    <img src="https://contrib.rocks/image?repo=posture-sec/guage" alt="Contributors" />
  </a>
</p>

## License

Apache-2.0. See `LICENSE`. This code is unaudited; see `SECURITY.md`.
