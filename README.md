# Gauge (`stellar-gauge`)

Status: scope-first scaffold only (ROADMAP task 1.12, decision D-09). Nothing audits an account yet.

Gauge will read the public configuration of Stellar accounts (signers, thresholds, flags, home domain) and compare it with declared expectations in `gauge.toml`. It reports posture: what the chain shows against what you declared. It does not say an account is secure or safe, and key custody and holder identity are invisible on-chain.

## Current state

The module, package boundaries, model and policy declarations and a non-operational CLI exist. `go run ./cmd/stellar-gauge` prints a scaffold notice and exits 3.

Not implemented: subcommands, policy loading, network clients, rules, sanitization, threshold math, renderers and exit-code mapping. See `ROADMAP.md` for scope and `project_structure.md` for where each feature will live.

## What it will not do

- No secret keys: a secret seed input is rejected without echoing it.
- No transaction building or submission, and no writes except an explicit `--out` file.
- No daemon, streaming, dashboard, plugin system or rule language.

## Roadmap rule

Every contribution updates `ROADMAP.md` in the same pull request: task boxes, a changelog row, the progress table and the `Last updated` line. See `AGENTS.md`.
