# project_structure.md: Gauge (working name)

Where everything lives, who may change it and which way imports may point. Read this before creating or moving a file. Decisions behind the layout: `ARCHITECTURE.md` section 5. Work queue: `ROADMAP.md`.

> **Roadmap rule (every contribution):** any change to this repository, by a human or an agent, must update `ROADMAP.md` in the same pull request: tick or add tasks, add a changelog entry, refresh the progress table and the `Last updated` line. If you add, move or remove a file, also update this document.

Status: Phase 3 complete on 2026-11-05, 7 of 7, with the facts report (3.7) as the last task the phase owed; Phase 2 closed on 2026-11-04 at 11 of 11. The scaffold-only exception D-09 was lifted by the owner on 2026-10-30. Module `github.com/posture-sec/guage`, binary `stellar-gauge`, Go 1.27.3, one approved dependency, existing Apache-2.0 license. Root documents are the maintained source of truth; the untouched `files/` inputs remain local and are not part of the scaffold commit. A file's existence does not mean its feature is implemented.

## 1. Annotated tree

Brace lists below name individual files. Every internal package also has `doc.go`. Unless marked as a declaration or test, Go files contain only a package clause. Empty directories have `.gitkeep` so they survive Git checkout.

```text
guage/
  AGENTS.md, CLAUDE.md                     # copied agent instructions
  PRD.md, ARCHITECTURE.md                  # product scope and target design
  ARCHITECTURE_ESSENTIALS.md               # hard rules and current implementation status
  ROADMAP.md, project_structure.md         # work queue and this scope map
  README.md                                # status, limits and roadmap rule (1.10)
  CONTRIBUTING.md, SECURITY.md             # process drafts carrying the roadmap rule (1.8)
  LICENSE                                 # existing Apache-2.0 license
  .gitignore                              # local policies, outputs, briefings and secrets stay untracked
  go.mod                                  # module identity, Go 1.27.3 and BurntSushi/toml v1.6.0 (ADR-004)
  go.sum                                  # checksums for the one approved module
  staticcheck.conf                        # enable all checks
  gauge.example.toml                      # the documented starting point: loads through the strict parser, and `audit` with no policy file names it (3.7)

  cmd/stellar-gauge/
    main.go                               # dispatch, exit-code mapping, report rendering and the deps seam (2.9, 3.5)
    {check,audit,version}.go              # P0 commands: flags, account validation, policy load (2.9)
    cli_test.go                           # in-process CLI tests: check, audit, JSON output, failure modes
    facts_test.go                         # the zero-config surface: fact lines, the example policy and the messages that name what to type (3.7)
    g3_test.go                            # fail-closed gate G3 cases (2.10)
    {snapshot,diff}.go                    # P1 command placeholders (5.2)

  internal/
    model/
      {account,balance,signer,network}.go  # observed-state declarations; balance exposure reserved for P1
      {finding,report,role,severity,exit}.go # findings, per-account reports and enums
      facts.go                            # the fact shapes a report prints ahead of its findings (3.7)
      model_test.go                       # severity names, fingerprint, ordering, JSON and exit-code contracts
    strkey/
      {address,checksum,seed}.go           # SEP-23 classify, CRC16 and seed detection (2.2)
      {address_test,fuzz_test}.go          # SEP-23 vectors, shape and sentinel tests; FuzzClassify
    threshold/
      {threshold,threshold_test}.go       # authorization math as verified in 0.7 (2.3)
    sanitize/
      {text,domain,markdown}.go            # output and domain handling (2.5, 5.3)
      {sanitize_test,fuzz_test}.go         # injection classes, IDN, fencing; FuzzText and FuzzFenced
    policy/
      {config,expectations,accept}.go      # policy declarations, no decoding or acceptance logic
      {decode,validate,roles}.go           # strict loading, rejection kinds, network passphrases and the Appendix A role floors (2.4, 3.4)
      config_test.go                       # loader tests, including that the shipped example policy loads (2.4)
      {accept,roles}_test.go               # acceptance refusals and the role-floor table (3.4)
    source/
      source.go                           # Source interface and the passphrase comparison
      {horizon,http,errors}.go             # Horizon client: caps, retries, strict decode (2.7)
      horizon_test.go                     # fake-server success and failure-mode tests (2.7)
    sep1/
      {fetch,guard,parse}.go               # guarded identity acquisition: hostname check, dial-time IP guard, caps (3.5)
      {fetch_test,guard_test,parse_test}.go # SSRF, rebinding, redirect, size, sanitizer-boundary and parse tests (3.5)
    rules/
      {rules,account,signers,thresholds,flags,identity}.go # registry and pure rule boundaries; identity and signers read the two role expectations 3.4 defines (3.1 to 3.5 done)
      facts.go                            # rules.Facts: the observations of one context, in the same pure shape as a rule and returning no finding (3.7)
      {rules,identity,signers,thresholds,facts}_test.go # positive and negative cases, guard proofs, a captured account and the two kinds of absence (3.1 to 3.7)
    report/
      {text,json,exit}.go                  # P0 renderers, the fact block ahead of the findings, limits footer and outcome mapping (2.6, 3.7)
      markdown.go                         # P1 output (5.3)
      report_test.go                      # golden files, sanitization, determinism, exit tests and the fact block in both formats
    run/
      {run,check,audit}.go                 # P0 orchestration: passphrase, bounded fetch, identity fetch, role severity, accepts, the fact block and `HasPolicy` (2.8, 3.4, 3.5, 3.7)
      {compare,snapshot,diff}.go           # P1 orchestration (5.1, 5.2)
      run_test.go                         # end-to-end orchestration with a fake source and an offline fetcher, the UTC expiry judgement and the two builders' fact shapes (2.8, 3.4, 3.5, 3.7)
      roles_test.go                       # one account at five roles: severity, fingerprint, accepts and the lock declaration (3.4)

  testdata/
    horizon/README.md                     # provenance of every file: which five are testnet captures, which seven are built, and how (0.6, 3.6)
    horizon/{root-testnet,1-default,7-extra-signer,8-clawback-undeclared,10-not-found}.json  # captures (0.6)
    horizon/{2-master-zero,3-unreachable-high,4-thresholds-out-of-order,5-hashx-signer-at-medium,6-preauth-signer-only,9-home-domain-present,12-hostile-home-domain}.json  # built for Appendix B (3.6)
    policy/.gitkeep                       # still empty: a scenario declares its policy as `policy.Account` values in `internal/run/scenarios_test.go`, and acceptance and role cases are inline TOML in `internal/policy`
    golden/{sample.text,sample.json}.golden # regenerated only with `-update` and reviewed (2.6)
    golden/<scenario>.{text,json}.golden  # one pair per Appendix B case, 36 files, compared by `internal/run/scenarios_test.go` (3.6, 3.7)

  docs/
    banner.svg                            # README banner; wordmark plus status line
    WAVE_SUBMISSION.md                    # verified Drips Wave facts, gate list, runbook        (1.15, D-13)

  site/                                   # documentation site v1, deployed to GitHub Pages      (5.9, 5.10)
    index.html                            # overview
    rules.html                            # rule reference
    policy.html                           # policy file reference
    mechanics.html                        # how a run works
    guides.html                           # clone to failing build
    status.html                           # what ships, what is verified, what does not
    404.html                              # Pages 404; root-relative asset paths
    assets/{site.css,site.js,favicon.svg,og.png}  # one stylesheet (dark tokens plus a light override), copy buttons and theme toggle, share card

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
      pages.yml                           # pages / deploy: publishes site/ to GitHub Pages        (5.9)
```

Remote state set up alongside these files (task 1.13, not visible in the tree): a `protect main` ruleset requiring a pull request and the three CI checks, the label taxonomy listed in `CONTRIBUTING.md`, repository topics, GitHub Discussions, and 73 issues (#5 to #77), one per open roadmap task in Phases 0 to 6 plus the ten readiness gates, produced by `scripts/generate_issues.sh`. The 1.3 direct-push rejection was observed against `main` itself on 2026-10-30, after PR #2 merged: an empty-commit push was refused with "Changes must be made through a pull request" and `main` did not move. GitHub Pages is enabled with `build_type: workflow`, so `site/` reaches `https://posture-sec.github.io/guage/` only through the `pages` workflow (5.9, verified 2026-11-02). Also set on the repository itself (1.17, 2026-11-02): the About description names what Gauge does and states that it never holds a key, the homepage field points at the documentation site, and the wiki is off (`has_wiki: false`, `/wiki` redirects to the repository) so `site/` is the only documentation surface; the nine topics are unchanged. On the org itself (1.17): name `Posture Security`, a one-line bio, the documentation site as its website, location `Remote`, and the contact email the owner chose to publish. Org-wide 2FA remains off because task 1.2 is the owner's to close. The org has a second, separate repository for its own front door: `posture-sec/.github`, public, default branch `main`, with issues, projects and wiki off, holding `profile/README.md`, which GitHub renders on `https://github.com/posture-sec`. That file is not in this tree and cannot be shipped from here. It said, until 2026-11-02, that Gauge was the only code repository published in this org; that is now false twice over. `posture-sec/airlock` exists: the sibling scanner moved into this organization that day after the account that owned it was suspended (Airlock's task 1.15). `posture-sec/stellar-account-fixtures` exists: the account-read corpus, written for the problem this repository's own 0.6 ran into, namely that a rule written against a hand-typed fixture passes until the first real account disagrees with the invention (its tasks 0.1 to 0.5 and 1.1 to 1.7). `.github` remains the front door rather than a product. Three program repositories, plus that front door, means one Drips Wave application path, one org-scope App install and one per-org point pool split across the applicants, which `docs/WAVE_SUBMISSION.md` 7 now covers. Where a fixture comes from the corpus rather than from a capture of Gauge's own is `ARCHITECTURE.md` 12 and section 6 of this file, and nothing from it is vendored here yet. Absent on purpose: no release tag (D-12), and no scheduled-audit or release workflow yet (4.5, 5.4).

Drafted but not finished: `README.md` (1.10) and `CONTRIBUTING.md`, `SECURITY.md`, `.github/pull_request_template.md` (1.8) carry the roadmap rule and status wording; each task is done only when merged.


Still absent, not silently implemented: a release workflow (4.5) and a scheduled-audit template (5.4). `go.sum` holds Go's own checksums for the one approved module (ADR-004, added in 2.4); no lockfile is hand-drafted. The local `files/` briefing copies are Git-ignored; the root documents are the maintained source of truth.

Checks that run today: `gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./... -race -count=1`, `staticcheck ./...`, `govulncheck ./...`, `scripts/check_roadmap_touched.sh origin/main`, `scripts/roadmap_progress.sh --check`, `scripts/test_roadmap_gate.sh`, `scripts/generate_issues.sh --check`. OSSF Scorecard runs on pushes to `main` and weekly (`scorecard / analysis`), not on pull requests, because it audits the default branch and publishes results. `stellar-gauge version` prints the version, and `check` and `audit` read real accounts through the Horizon client, fetch the `stellar.toml` of any account that publishes a home domain through the guarded client, evaluate the sixteen registered rules and render a report; a HIGH finding exits 1 by default, which since 3.1 includes an unreachable threshold tier, a signer the policy does not declare and an account with no standing weight, and the WARN family (flag or threshold drift, an out-of-order tier, a `hash(x)` signer, an unreadable identity) exits 1 under `--fail-on warn`. The implemented packages are `model`, `strkey`, `threshold`, `sanitize`, `policy`, `source`, `sep1`, `report`, `rules`, `run` and `cmd/stellar-gauge` (2.1 to 2.11, 3.1 to 3.7), and `strkey` and `sanitize` carry fuzz targets. `internal/threshold` answers reachability and the minimum signer count, `internal/rules` holds all sixteen rules, and `internal/rules/facts.go` has carried `rules.Facts` since 3.7 landed on 2026-11-05: the same context a rule reads, turned into the observations a report prints ahead of its findings, with no finding in it and no path to an exit code.

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
- `rules` may import `model`, `threshold`, `policy` types and `sep1` result types; today its direct imports are `model`, `policy`, `sep1`, `fmt` and `strings`. It never imports `source`, `net/http` or any I/O package, and no function in it performs one: an identity rule reads the `*sep1.Result` the caller placed in its context. `go list -deps` shows `net/http` under `rules` because `sep1`'s result types and its client share a package, which is why the purity check is on imports.
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
| Add a fixture | `testdata/horizon/<scenario>.json` | Note its origin in `testdata/horizon/README.md`: own capture, one of `posture-sec/stellar-account-fixtures` with its sidecar digest and ledger, or built here and labelled as built, and whether it is perishable (testnet) |
| Change a hard rule or decision | `ARCHITECTURE_ESSENTIALS.md`, `ARCHITECTURE.md` section 13 | New ADR row; owner approval |
| Record a decision | `ROADMAP.md` decision log | |

## 5. Naming conventions

- Rule IDs: kebab-case with a family prefix: `acct-`, `sig-`, `thr-`, `flag-`, `id-`, `src-`. Stable once released; never reused.
- Packages: short, lowercase, one word. Files: lowercase with underscores only when needed.
- Fixtures: `testdata/horizon/<scenario>.json` matching the numbered scenarios in `PRD.md` Appendix B; golden files `<scenario>.<format>.golden`.
- Tests: `_test.go` beside the code; fuzz targets named `FuzzXxx`; opt-in network tests carry the build tag `livenet`.
- Commits: Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`), subject under 72 characters.

## 6. Fixtures

- Real captured account JSON is preferred. Account data is public, so captures are safe to commit unedited. Record the network, date and ledger in a sidecar notes file. Where no capture can exist, because the shape needs a transaction submission this project forbids, the fixture is built here, labelled as built in `testdata/horizon/README.md`, and never described as an observation: it proves the decoder and the rules, not that the chain served the shape.
- `posture-sec/stellar-account-fixtures` (public since 2026-11-02) is where a mainnet account shape comes from: nine `GET /accounts/<id>` reads re-serialised with sorted keys, eight on mainnet and one on testnet since 2026-11-03, each beside a sidecar whose SHA-256, capture time, ledger and Horizon version `validate.py` recomputes. A fixture copied from it keeps that fixture's account id, `last_modified_ledger` and sidecar digest in the notes file so the claim survives the copy. Nothing is vendored in `testdata/` yet. Two shapes are in neither capture set rather than three: `hash(x)` and pre-auth signer entries, where every signer key in either set starts with `G`, and both stay hand-built because bringing one into existence needs a transaction submission this project forbids. The narrower claim that a master key at weight 0 cannot be found was never true: four reads show the master present at weight 0 among other signers (`testdata/horizon/7-extra-signer.json` with six signers, plus the corpus's `coordinator-data-entry`, `sponsored-trustlines` and `testnet-multisig`), so `sig-no-signing-weight` and the weight-0 branch of the threshold math have real bytes behind them. Appendix B scenario 2 is a different account: master weight 0 with no other signer, which nobody can sign for, still uncaptured here and in the corpus, where `master_absent` and `no_signers` are open gaps.
- Testnet resets can delete accounts; mark testnet fixtures perishable and keep them as static files, never refetched by tests.
- Hostile-string fixtures (ANSI, bidi, markdown) are built from fragments rather than raw bytes. Markdown cases do that in test code; `12-hostile-home-domain.json` does it in data, holding the escape and the override as JSON `\u` sequences so no control byte is in the file while the decoded value still carries one, which is the only way a golden can pin what the report prints.
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
