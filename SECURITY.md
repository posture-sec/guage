# Security policy

## Reporting a vulnerability

Report it privately through [a security advisory](https://github.com/posture-sec/guage/security/advisories/new). Do not open a public issue. Expect an acknowledgement within seven days while this is a one-maintainer project.

Include the command you ran, the exit code, and the policy file with real account IDs left in. Account configuration is public data, so pasting it is safe. **Never paste a secret seed (`S...`) into a report, an issue or this CLI.** If a seed did reach a file or shell history, rotate the key first, then report.

## Scope

In scope:

- A Gauge run that accepts, stores, logs or echoes a secret seed.
- Any transaction building or submission, or any write other than to the file an explicit `--out` names. That flag is reserved for the P1 `snapshot` command, so every shipped subcommand writes nothing but stdout and stderr.
- A network request without a timeout, a response size cap, bounded retries and bounded concurrency.
- `stellar.toml` reached without the dial-time SSRF guard, or a redirect chain that is not re-validated.
- Chain-derived data reaching terminal or markdown output unsanitized (control, bidi, zero-width characters, terminal escapes).
- Exit code 0 after an unreachable endpoint, an oversized or malformed response, an unknown config key, a wrong network passphrase or a panic. Everything fails closed.
- A rule that reports a pass because data was missing rather than because it was checked.

Out of scope:

- Findings about the accounts you audit. Gauge describes chain configuration; it does not evaluate key custody.
- Vulnerabilities in Horizon, Stellar Core, Soroban or the public endpoints Gauge reads.
- Rate limiting, availability and cost of the public endpoints.
- Reports from a build of a fork or an untagged commit, unless the issue also reproduces on `main`.

## Status of this code

**Unaudited.** No third party has reviewed Gauge. `govulncheck` and `staticcheck` run in CI on every pull request, and the three fuzz targets over the sanitizer and strkey paths are not CI jobs: they are short local smoke runs a contributor makes before a release-facing change (`AGENTS.md` section 5), last run clean on 2026-11-03 for ten seconds each. Neither the CI checks nor the smokes are a review.

**Functional alpha, unaudited.** As of the current commit there is a policy loader, a Horizon client with timeouts, size caps and bounded retries, strict decoding, a guarded `stellar.toml` fetcher whose dial-time address check is the only thing between a chain-derived home domain and a socket, report renderers, seven account, flag and identity rules, an orchestrator and a CLI. The signer and threshold rules are not implemented, so a run reports observed state plus those seven and cannot evaluate the rest of the catalog. The code has not been independently reviewed, so nothing here should be relied on to detect anything. See `ROADMAP.md` for what is actually implemented.

## What this tool cannot tell you

Gauge reports posture: what the chain shows compared with what you declared. It never says an account is secure or safe, and a clean run is not an approval.

- Key custody is invisible on-chain. A threshold that requires three signers tells you nothing about whether those three keys are held by three independent people or by one laptop.
- The policy file is the definition of correct. Anyone who can edit both `gauge.toml` and the workflow that runs it can make a check pass. Protect the policy with review and branch protection; that residual risk is documented in `ARCHITECTURE_ESSENTIALS.md` section 10 and is not solvable in code.
- A home domain that resolves and lists your account is a claim about intent, not proof of identity.
