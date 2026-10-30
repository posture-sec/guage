# Contributing to Gauge

Gauge is a read-only audit tool, so the bar for a change is "it cannot produce a false pass". Read `ARCHITECTURE_ESSENTIALS.md` section 2 before you write code; those rules are not negotiable, and `AGENTS.md` section 12 lists the paths that need extra care.

## Status

The repository is a scaffold (`ROADMAP.md` task 1.12). Package directories and placeholder files mark scope boundaries. They are not implemented features, and a file's existence is not progress. Pick work from the open issues or from `ROADMAP.md` and say which task you are taking.

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
```

CI runs the same set as `ci / go`, `ci / vuln` and `roadmap-check / gate`, with every action pinned by commit SHA and `permissions: contents: read`. Fuzz smoke tests exist once `internal/strkey` and `internal/sanitize` have targets:

```bash
go test ./internal/strkey ./internal/sanitize -run '^$' -fuzz=Fuzz -fuzztime=10s
```

Tests never touch the real network. A test that needs live Horizon carries the `livenet` build tag and is opt-in.

## Issues and labels

Issues carry one `area:` and one `type:` label, plus `complexity:` when the task has a roadmap size. They are scoped to a single task or rule.

The planned backlog is generated, not hand-maintained: `scripts/generate_issues.sh` reads the open tasks in `ROADMAP.md` and opens one issue per task and per readiness gate, with the acceptance criteria taken from the task's "Done when" line. It is idempotent, so rerunning it after a roadmap change only adds what is missing; `--update` rewrites the body and labels of issues that already exist when the roadmap line moved. `--check` fails when an open task has no row in the script, which is the reminder to add one in the same pull request. Gate issues carry no `complexity:` label because gates have no roadmap size.

`complexity:` mirrors the roadmap size, one to one: `S` becomes `complexity: trivial`, `M` becomes `medium`, `L` and `XL` become `high`. The labels match the tiers a funding program reads, so changing a roadmap size and rerunning the generator is the only step.

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
