# Contributing to Gauge

Gauge is a read-only audit tool, so the bar for a change is "it cannot produce a false pass". Read `ARCHITECTURE_ESSENTIALS.md` section 2 before you write code; those rules are not negotiable, and `AGENTS.md` section 12 lists the paths that need extra care.

## Status

The repository is a functional alpha (`ROADMAP.md` Phase 2): the policy loader, Horizon client, report renderers, orchestrator and CLI are implemented and tested. The threshold math, the rules and the SEP-1 fetcher are still placeholders, and a placeholder file is not progress. Pick work from the open issues or from `ROADMAP.md` and say which task you are taking.

## Before you start

1. Find the task in `ROADMAP.md`. If it does not exist, add it with the next free ID in its phase and explain why.
2. Confirm its dependencies are done. If they are not, stop and say so in the issue.
3. If the task needs a new Go module, a new network call, or a change to a protected path that loosens a check, ask first. The only permitted third-party dependency is `github.com/BurntSushi/toml`.

## Workflow

1. Branch from `main`: `feat/<task-id>-<slug>`. Never commit to `main`; the branch is protected by a ruleset that requires a pull request and passing checks.
2. Write the failing test or fixture first. Run it and watch it fail for the right reason.
3. Implement the smallest change that passes.
4. Run the checks locally (see the next section) and paste the real output in the pull request.
5. Update the docs the change touches. `project_structure.md` for any added, moved or removed file.
6. Update `ROADMAP.md` in the same pull request: task boxes, a changelog row with the UTC date, the progress table and the `Last updated` line. Run `scripts/roadmap_progress.sh --check` to regenerate the table rather than editing numbers by hand.
7. Open one pull request per task or per tightly related group. Do not merge your own PR without the checks going green.

## Checks

```bash
gofmt -l .                                   # must print nothing
go vet ./...
staticcheck ./...                            # v0.8.2, config in staticcheck.conf
go test ./... -race -count=1
govulncheck ./...                            # v1.9.0
scripts/check_roadmap_touched.sh origin/main
scripts/roadmap_progress.sh --check ROADMAP.md
scripts/test_roadmap_gate.sh                 # self-test for the roadmap gate
scripts/test_issue_shape.sh                  # every generated body matches PI-1
```

CI runs the same set as `ci / go`, `ci / vuln` and `roadmap-check / gate`, with every action pinned by commit SHA and `permissions: contents: read`. The fuzz targets in `internal/strkey` and `internal/sanitize` exist and are run as a smoke before a release-facing change, one target per command because `-fuzz` refuses multiple packages. They are not CI jobs:

```bash
go test ./internal/strkey -run '^$' -fuzz=FuzzClassify -fuzztime=10s
go test ./internal/sanitize -run '^$' -fuzz=FuzzText -fuzztime=10s
go test ./internal/sanitize -run '^$' -fuzz=FuzzFenced -fuzztime=10s
```

Tests never touch the real network. A test that needs live Horizon carries the `livenet` build tag and is opt-in.

## Issues and labels

Every issue here is written to **PI-1**, the organization's issue contract in
[`posture-sec/.github`](https://github.com/posture-sec/.github/blob/main/ISSUE_CONTRACT.md): one marker
line, one sentence of intent, then nine numbered sections in a fixed order. The format exists so an
issue can be picked up cold, by someone who has never seen this repository and cannot ask.

The two rules the contract is built on: `ROADMAP.md` is the source of truth and an issue copies a row
rather than replacing it, so a pull request that disagrees with the roadmap fixes the roadmap first;
and a claim names the run that produced it or says it is local, which is why every issue separates
what a desk can prove from what needs the hosted half.

Issues carry one `area:` and one `type:` label, plus `complexity:` when the task has a roadmap size.
They are scoped to a single task or rule.

The planned backlog is generated, not hand-maintained: `scripts/generate_issues.sh` reads the open
tasks in `ROADMAP.md` and builds one PI-1 body per task and per readiness gate. Nine sections come
from two places. Seven are derived from the roadmap row, the area label and the repository's own rule
table. Two cannot be derived, section 1 and section 7, so each task carries a curated prose row in
`issue_prose` with its kind, its one-sentence brief, its why and its out-of-scope. A roadmap row with
no prose row gets no body: `--check` fails and the generator refuses to emit, which is how a new task
is stopped from minting an empty issue. Add the prose row in the same pull request that adds the row.

The script is idempotent, so rerunning it after a roadmap change only adds what is missing; `--update`
rewrites the body and labels of issues that already exist when the roadmap line moved. `--check` fails
when an open task has no row in the script or no prose, which is the reminder to add both in the same
pull request. Gate issues carry no `complexity:` label because gates have no roadmap size, and their
section 2 asks for an observation rather than a diff: a gate closes on a reading, and nobody should
tick one by shipping code that claims it holds.

`scripts/test_issue_shape.sh` builds every open task's body and checks the shape: the marker form, the
nine headings in order, the roadmap tick box, the local and hosted halves, no em dash, lines wrapped
near 100 columns, and the length band the contract sets. It runs in CI as part of `roadmap-check /
gate`.

`complexity:` mirrors the roadmap size, one to one: `S` becomes `complexity: trivial`, `M` becomes
`medium`, `L` and `XL` become `high`. The generated marker carries the same value in its `wave:` field
for the rows that are wave work. Changing a roadmap size and rerunning the generator is the only step
for the label; the tier a contributor is paid at is entered in the program dashboard, not derived from
a label.

| Prefix | Meaning |
| --- | --- |
| `area: model`, `area: policy`, `area: source`, `area: sep1`, `area: rules`, `area: report`, `area: run`, `area: strkey`, `area: threshold`, `area: sanitize`, `area: cli`, `area: ci`, `area: docs` | Which package the work lands in |
| `complexity: trivial`, `complexity: medium`, `complexity: high` | Relative effort, from the roadmap size, not a time estimate |
| `type: feature`, `type: test`, `type: tooling`, `type: docs`, `type: security`, `type: research` | Kind of work |
| `good first issue`, `help wanted` | Contributor routing: the dependency-free research tasks that need reading and writing rather than a decision from the owner |

Do not add a `Stellar Wave` label to an issue to enroll it in a funding cycle. The program docs state that issues tagged through GitHub are assigned Trivial complexity and 100 points whatever the work turns out to be; the tier belongs in the dashboard, under Maintainers, so the maintainer can set it. `docs/WAVE_SUBMISSION.md` holds the verified program facts and the runbook.

## Adding a rule

1. A function in `internal/rules/<family>.go`, registered in `rules.go`. Rules are pure: no I/O, no globals, no time, no randomness.
2. A positive and a negative fixture in `testdata/`, plus golden output updates (`-update`, then review the diff).
3. `PRD.md` section 6.1 and `ARCHITECTURE_ESSENTIALS.md` section 8 list the rule with its severity and rationale.
4. Rule IDs are kebab-case with a family prefix (`acct-`, `sig-`, `thr-`, `flag-`, `id-`, `src-`), stable once released, never reused.

## Writing style

Docs, comments and commit messages use plain, direct language. Output describes posture and never says "secure" or "safe". Comments explain why, not what. Conventional Commits, subject under 72 characters, no em dashes.
