# Security policy

Status: draft for a scaffold (ROADMAP 1.12, D-09). There is no released binary, so no version is currently supported.

## Reporting

Report a vulnerability in Gauge privately through the repository security advisory process on `posture-sec/guage`. Do not open a public issue for a security report.

Expect an acknowledgement within seven days while the project has one maintainer.

## What this tool is not

Gauge reports account posture: what the chain shows compared with declared expectations. It never says an account is secure or safe. Key custody, holder identity and off-chain compromise are outside what on-chain data can show.

It is read-only and secret-free. It never accepts, stores or echoes a secret seed, builds or submits transactions, or writes anything except to an explicit `--out` file.

## Trust boundary and residual risk

Everything fetched is treated as hostile: Horizon responses, `stellar.toml` content and every string inside them, including home domains and data entries. Output is sanitized and network access is bounded.

The configuration file is policy. An attacker who can edit both `gauge.toml` and the scheduled workflow that runs Gauge can make a check pass; that is a process problem, not something this tool can solve in code. Protect the policy with code review and branch protection, and store baselines outside the audited repository.
