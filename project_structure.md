# project_structure.md: Gauge (working name)

Where everything lives, who may change it and which way imports may point. Read this before creating or moving a file. Decisions behind the layout: `ARCHITECTURE.md` section 5. Work queue: `ROADMAP.md`.

> **Roadmap rule (every contribution):** any change to this repository, by a human or an agent, must update `ROADMAP.md` in the same pull request: tick or add tasks, add a changelog entry, refresh the progress table and the `Last updated` line. If you add, move or remove a file, also update this document.

Status: scope-first scaffold (ROADMAP 1.12, D-09). Module `github.com/posture-sec/guage`, binary `stellar-gauge`, Go 1.27.3, existing Apache-2.0 license. Root documents are the maintained source of truth; the untouched `files/` inputs remain local and are not part of the scaffold commit. A file's existence does not mean its feature is implemented.

## 1. Annotated tree

Brace lists below name individual files. Every internal package also has `doc.go`. Unless marked as a declaration or test, Go files contain only a package clause. Empty directories have `.gitkeep` so they survive Git checkout.

```text
guage/
  AGENTS.md, CLAUDE.md                     # copied agent instructions
  PRD.md, ARCHITECTURE.md                  # product scope and target design
  ARCHITECTURE_ESSENTIALS.md               # hard rules and current implementation status
  ROADMAP.md, project_structure.md         # work queue and this scope map
  README.md                                # scaffold status, limits and roadmap rule (1.10 draft)
  CONTRIBUTING.md, SECURITY.md             # process drafts carrying the roadmap rule (1.8)
  LICENSE                                 # existing Apache-2.0 license
  .gitignore                              # local policies, outputs, briefings and secrets stay untracked
  go.mod                                  # module identity and Go 1.27.3; no dependencies yet
  staticcheck.conf                        # enable all checks
  gauge.example.toml                      # commented, intentionally unconfigured policy draft

  cmd/stellar-gauge/
    main.go                               # scaffold notice, exit 3, no input processing or I/O beyond stderr
    {check,audit,version}.go               # P0 command placeholders (2.9)
    {snapshot,diff}.go                     # P1 command placeholders (5.2)

  internal/
    model/
      {account,balance,signer,network}.go  # observed-state declarations; balance exposure reserved for P1
      {finding,report,role,severity,exit}.go # findings, per-account reports and enums
      model_test.go                       # severity names, fingerprint, ordering, JSON and exit-code contracts
    strkey/
      {address,checksum,seed}.go           # SEP-23 classify, CRC16 and seed detection (2.2)
      {address_test,fuzz_test}.go          # SEP-23 vectors, shape and sentinel tests; FuzzClassify
    threshold/
      {threshold,threshold_test}.go       # authorization math pending verified semantics (2.3)
    sanitize/
      {text,domain,markdown}.go            # output and domain handling (2.5, 5.3)
      {sanitize_test,fuzz_test}.go         # injection classes, IDN, fencing; FuzzText and FuzzFenced
    policy/
      {config,expectations,accept}.go      # policy declarations, no decoding or acceptance logic
      {decode,validate,roles}.go           # strict loading and role profiles (2.4, 3.4)
      {config_test,accept_test}.go         # reserved tests
    source/
      source.go                           # context-aware Source interface only
      {horizon,http,errors}.go             # source implementation and failure behavior (2.7)
      horizon_test.go                     # reserved fake-server tests
    sep1/
      {fetch,guard,parse}.go               # guarded identity acquisition; no networking yet (3.5)
      {fetch_test,guard_test}.go           # reserved SSRF and parser tests
    rules/
      {rules,account,signers,thresholds,flags,identity,facts}.go # pure rule boundaries (3.1 to 3.7)
      rules_test.go                       # reserved positive and negative cases
    report/
      {text,json,exit}.go                  # P0 renderers and outcome mapping (2.6)
      markdown.go                         # P1 output (5.3)
      report_test.go                      # reserved golden tests
    run/
      {run,check,audit}.go                 # P0 orchestration (2.8)
      {compare,snapshot,diff}.go           # P1 orchestration (5.1, 5.2)
      run_test.go                         # reserved integration tests

  testdata/
    horizon/README.md                     # capture provenance, ledgers, what could not be captured (0.6)
    horizon/{root-testnet,1-default,7-extra-signer,8-clawback-undeclared,10-not-found}.json
    policy/.gitkeep                       # valid/invalid loader fixtures pending (2.4)
    golden/.gitkeep                       # report fixtures pending (2.6, 3.6, 5.3)

  docs/
    banner.svg                            # README banner; wordmark plus status line
    WAVE_SUBMISSION.md                    # verified Drips Wave facts, gate list, runbook        (1.15, D-13)

  scripts/
    check_roadmap_touched.sh              # gate: a PR that changes files must change ROADMAP.md   (1.9)
    roadmap_progress.sh                   # computes the progress table; --check fails when stale   (1.9)
    test_roadmap_gate.sh                  # eight cases for both scripts, on throwaway git repos    (1.9)
    generate_issues.sh                    # roadmap to GitHub issues; --check, --apply, --update   (1.13, 1.14)

  .github/
    CODEOWNERS                            # owner on every path, extra weight on the protected ones (1.3)
    dependabot.yml                        # gomod and actions, grouped weekly                       (1.6)
    pull_request_template.md              # includes the ROADMAP checkbox                           (1.8)
    ISSUE_TEMPLATE/
      config.yml                          # routes security reports away from issues
      bug.yml                             # bug form: command, exit code, policy excerpt, version
    workflows/
      ci.yml                              # ci / go (gofmt, vet, build, test -race, staticcheck) and ci / vuln (1.5)
      roadmap-check.yml                   # roadmap-check / gate: touched, table, backlog rows, self-test (1.9, 1.14)
      scorecard.yml                       # scorecard / analysis: OSSF Scorecard, SARIF and badge  (1.7)
```

Remote state set up alongside these files (task 1.13, not visible in the tree): a `protect main` ruleset requiring a pull request and the three CI checks, the label taxonomy listed in `CONTRIBUTING.md`, repository topics, GitHub Discussions, and 73 issues (#5 to #77), one per open roadmap task in Phases 0 to 6 plus the ten readiness gates, produced by `scripts/generate_issues.sh`. The 1.3 direct-push rejection was observed against `main` itself on 2026-10-30, after PR #2 merged: an empty-commit push was refused with "Changes must be made through a pull request" and `main` did not move. Absent on purpose: no release tag (D-12), and no scheduled-audit or release workflow yet (4.5, 5.4).

Drafted but not finished: `README.md` (1.10) and `CONTRIBUTING.md`, `SECURITY.md`, `.github/pull_request_template.md` (1.8) carry the roadmap rule and status wording; each task is done only when merged.


Still absent, not silently implemented: a release workflow (4.5), a scheduled-audit template (5.4), the Scorecard workflow (1.7) and `go.sum`. `go.sum` will be generated by Go only after the approved TOML dependency is reviewed and added (0.13); no lockfile is hand-drafted. The local `files/` briefing copies are Git-ignored; the root documents are the maintained source of truth.

Checks that run today: `gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./... -race -count=1`, `staticcheck ./...`, `govulncheck ./...`, `scripts/check_roadmap_touched.sh origin/main`, `scripts/roadmap_progress.sh --check`, `scripts/test_roadmap_gate.sh`, `scripts/generate_issues.sh --check`. OSSF Scorecard runs on pushes to `main` and weekly (`scorecard / analysis`), not on pull requests, because it audits the default branch and publishes results. `go run ./cmd/stellar-gauge` still prints the scaffold notice and returns nonzero. No subcommand, policy loader, network client, audit rule or renderer is operational. The strkey, sanitize and model packages are implemented and fuzzed (2.1, 2.2, 2.5); tests beside the remaining empty packages are file placeholders, not evidence of feature coverage.

Not built on purpose (do not create without a roadmap decision): a watch daemon, alert integrations, a rules language or plugin loader, a web UI, reputation scoring, contract-account auditing. See `ARCHITECTURE.md` 14.3.

## 2. Protected paths

Changes here are security-relevant. They need CODEOWNERS review and extra care from agents. Agents must not weaken any check in these paths without an explicit instruction from the owner.

| Path | Why |
|---|---|
| `internal/source/`, `internal/sep1/` | All network behavior: size caps, retries, SSRF guard |
| `internal/policy/`, `internal/strkey/` | Secret-seed rejection, strict config |
| `internal/sanitize/` | Output injection defense |
| `internal/threshold/` | Authorization math; an error here gives false assurance |
| `go.mod`, `go.sum` | Supply chain; any change needs a stated reason |
| `.github/workflows/`, `scripts/` | CI integrity and the roadmap gate |
| `.github/CODEOWNERS` | Review enforcement |

## 3. Import rules (dependency direction)

```
model  <-  strkey, threshold, sanitize  <-  policy, source, sep1  <-  rules  <-  report  <-  run  <-  cmd
```

- `model` imports nothing from this module.
- `rules` imports `model`, `threshold`, `policy` types and `sep1` result types. It never imports `source`, `net/http` or any I/O package.
- `report` imports only `model` and `sanitize`.
- Only `source` and `sep1` perform network I/O. Only `cmd`, `run` and `report` write to stdout, stderr or files.
- No `os/exec` anywhere. No `unsafe`. No `init()` with side effects. No package-level mutable state. No import cycles.
- The only allowed third-party import is the TOML parser, and only inside `policy` and `sep1`.

## 4. Where do I make a change?

| I want to | Edit | Also do |
|---|---|---|
| Add a rule | `internal/rules/<family>.go`, register in `rules.go` | Positive and negative fixtures in `testdata/`, a golden update, `PRD.md` 6.1, `ARCHITECTURE_ESSENTIALS.md` 8 |
| Change a severity by role | `internal/policy/roles.go` | `PRD.md` Appendix A, tests |
| Change threshold math | `internal/threshold/` | Table tests, brute-force cross-check, task 0.7 evidence; protected |
| Add a config field | `internal/policy/config.go` | Strict-decode tests, `gauge.example.toml`, `ARCHITECTURE.md` 7, bump `version` if breaking |
| Add an output format | `internal/report/<name>.go` | Sanitizer tests, golden files |
| Change HTTP behavior | `internal/source/` or `internal/sep1/` | Fake-server tests for the new behavior; protected |
| Add a fixture | `testdata/horizon/<scenario>.json` | Note its origin and whether it is perishable (testnet) |
| Change a hard rule or decision | `ARCHITECTURE_ESSENTIALS.md`, `ARCHITECTURE.md` section 13 | New ADR row; owner approval |
| Record a decision | `ROADMAP.md` decision log | |

## 5. Naming conventions

- Rule IDs: kebab-case with a family prefix: `acct-`, `sig-`, `thr-`, `flag-`, `id-`, `src-`. Stable once released; never reused.
- Packages: short, lowercase, one word. Files: lowercase with underscores only when needed.
- Fixtures: `testdata/horizon/<scenario>.json` matching the numbered scenarios in `PRD.md` Appendix B; golden files `<scenario>.<format>.golden`.
- Tests: `_test.go` beside the code; fuzz targets named `FuzzXxx`; opt-in network tests carry the build tag `livenet`.
- Commits: Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`), subject under 72 characters.

## 6. Fixtures

- Real captured account JSON is preferred. Account data is public, so captures are safe to commit unedited. Record the network, date and ledger in a sidecar notes file.
- Testnet resets can delete accounts; mark testnet fixtures perishable and keep them as static files, never refetched by tests.
- Hostile-string fixtures (ANSI, bidi, markdown) are built in test code from fragments.
- Never place a secret seed, even a testnet one, in the repository. Tests that need a seed-shaped string build one from fragments at runtime.

## 7. Documentation map and sync rules

| Document | Purpose | Update when |
|---|---|---|
| `PRD.md` | Scope, users, requirements | A requirement, priority or metric changes |
| `ARCHITECTURE.md` | Design and decisions | Behavior, models or ADRs change |
| `ARCHITECTURE_ESSENTIALS.md` | Short outline for agents; keep under about 150 lines | Any hard rule, decision or rule list changes |
| `ROADMAP.md` | Work queue, progress, changelog | **Every contribution** |
| `AGENTS.md`, `CLAUDE.md` | Agent instructions | Workflow, commands or boundaries change |
| `project_structure.md` | This map | A file or directory is added, moved or removed |
| `README.md`, `CONTRIBUTING.md` | Users and contributors | Behavior or process changes |

If a code change contradicts a doc, fix the doc in the same PR. If a doc statement is [Unverified], do not build on it; run the Phase 0 task that verifies it.

## 8. Generated and derived files

- The progress table in `ROADMAP.md` is computed by `scripts/roadmap_progress.sh`; regenerate it instead of editing numbers by hand.
- Golden files are regenerated only with an explicit `-update` flag and reviewed like code.
