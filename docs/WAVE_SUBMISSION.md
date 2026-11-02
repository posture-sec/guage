# Drips Wave submission packet: Gauge

Working notes for applying `posture-sec/guage` to a Stellar Wave cycle. Program facts below were read from the sources listed in section 6 on 2026-10-30 and re-read on 2026-11-02, not recalled. Nothing in this file claims the repository is approved; it is not. Section 7 covers the second repository the organization now has, because it changes what "applying" means.

## 1. Where the program stands

- Drips Wave launched with the Stellar ecosystem in January 2026, with waves typically running one week per month.
- Re-read 2026-11-02: the Stellar Wave page still shows ten waves concluded and states "There are no active or upcoming Waves at the moment", with budgets of $75000 and $60000 shown. A cycle window in the 2026-10-30 page data ran from 2026-10-15T13:00:46Z to 2026-10-22T13:00:56Z.
- Approved repositories on that page: 839 on 2026-10-30, 838 on 2026-11-02. The count moves by one between reads, which is worth stating so nobody reads a stale number as a trend. The sample on the first page includes early-stage repos alongside funded protocols, so an unbuilt product is not automatically out of scope.
- Not verified: whether this repository is already on that list. The only reliable check is the Maintainers dashboard after signing in with GitHub.
- Changed since the first draft of this file: `posture-sec` is no longer a one-repository organization. `posture-sec/airlock` arrived on 2026-11-02 (section 7), so the per-org budget and cap questions now have two applicants drawing on one pool.

## 2. Why this repository is not applying yet

One hard reason and one polish gap.

1. No cycle is open, so there is nothing to apply into today.
2. The rule set is thin. As of 2026-10-30 `stellar-gauge check` reads a real account through the Horizon client and evaluates four rules, so exit `1` is reachable: a missing account and a wrong network are HIGH findings, and declared flag drift and an undeclared clawback are warnings under `--fail-on warn`. Signer, threshold and identity rules plus role-based severities are still open (3.1, 3.4, 3.5), so a demo shows account and flag findings rather than the full rule catalog. A submission is stronger once a signer rule family lands, and the downside is asymmetric: a rejected repository cannot simply be re-applied, appeals go through the dashboard, are capped at three, and carry waiting periods.

## 3. Gate list before clicking Apply

- [x] PR #2 merged, so `main` carries CI, the roadmap gate, `README.md`, `CONTRIBUTING.md`, `SECURITY.md`, `CODEOWNERS`, issue forms and the banner. (merged 2026-10-30 as `ae6a05d`; the first push-event CI run on `main` passed)
- [x] Task 1.3 closed by observing the direct-push refusal against `main` itself, not a probe branch. (refused 2026-10-30 with "Changes must be made through a pull request", `main` unchanged)
- [x] Task 1.7 done: a Scorecard run where Branch-Protection returns a score. (2026-10-30: score 4, overall 6.2, Scorecard v5.6.0, badge live in the README. The three warnings about required approvals are accepted under D-14 because the repository has one maintainer, and they will keep the score down until a second reviewer exists.)
- [x] Task 2.9 done, so `stellar-gauge check` reads a real account and prints a report. (2026-10-30: the CLI, orchestration and Horizon client are merged and tested; gate G3 passes. Tasks 3.2 and 3.3 then registered four rules, so the report can carry findings and exit 1.)
- [ ] A short screen recording of that run end to end, which the playbook treats as the difference between a submission and an unfinished one.
- [ ] Task 0.4 answered with named operators, since the pitch rests on people who would run this on a schedule.
- [ ] Task 4.2 at least partly done: a rule reference a contributor can work from without reading Go source.
- [ ] The per-user and per-org repository caps for the cycle checked before deciding what to submit. These are not published anywhere in the maintainer docs; the dashboard is the only source. As of 2026-11-02 the organization has two candidate repositories (section 7), so this check decides between them rather than only gating this one, and a rejected application consumes the cap without refunding it.

## 4. Maintainer runbook for the cycle

Quoted panel names are from the Drips maintainer docs.

1. Sign in to the wave portal with the GitHub account that can administer the `posture-sec` organization. A phone number may need verifying before participating in a wave.
2. Install the Drips Wave GitHub App on the organization hosting your repositories. Organization scope matters: installing it on one repository only will not sync the others.
3. Under Maintainers, Orgs and Repos, sync and apply the repository to the Stellar Wave program. Applications require approval from the wave organizers.
4. If rejected, do not re-apply through the same route: appeal from the Orgs and Repos panel after the work has visibly improved.
5. Under Maintainers, Issues, add each issue with Add to Wave and set its tier there. The three tiers are Trivial, Medium and High. Do not use the GitHub `Stellar Wave` label as the shortcut: issues added that way are automatically Trivial at 100 points regardless of the real work, which would misprice 28 Medium and 3 High tasks in this backlog.
6. During the active week, check the dashboard daily to review applications and assign contributors.
7. Merge pull requests promptly and mark issues Resolved before the wave ends, or contributors lose their points. Unfinished work carries into the next cycle.

## 5. Draft submission answers

Refresh every number here on the day of filing; they were counted on 2026-10-30.

**Project description.** Gauge is a command line tool that reads the public configuration of your own Stellar accounts and compares it with what you said you expect. On Stellar, control of an account is a set of weights and thresholds stored on-chain: who can sign, how many signatures a high-value operation needs, which flags the issuer turned on. Those settings are public, rarely reviewed, and the reason accounts get locked or quietly gain a signer. Gauge fetches an account through Horizon, checks it against a small declared policy file, and exits nonzero when reality and intent disagree, so a scheduled run in CI catches a change to a treasury or issuer account the day it happens. It is read-only, never accepts a secret key, and treats every value it downloads as hostile input. The scale argument: Stellar averaged about 4.9 million daily transactions and roughly 63,000 daily active addresses in Q2 2026, with tokenized real-world assets on the network past $3 billion, and the accounts holding much of that are configured by people with no security review process.

**Repository description.** One Go module, `github.com/posture-sec/guage`, binary `stellar-gauge`. No smart contracts and no frontend, so there is no deployment to verify and no app URL: the deliverable is a static binary and a report. Dependencies are the standard library plus one reviewed TOML parser. Layered packages with a declared import direction, rules as pure functions, and a roadmap file that CI enforces on every pull request.

**Planned issues.** 73 open issues, generated from `ROADMAP.md` by `scripts/generate_issues.sh`, so each one is a roadmap task with its acceptance criteria taken from the task's own "Done when" line. Tiers as labelled: 32 Trivial, 28 Medium, 3 High, plus 10 readiness gates that are not wave work. By area: docs 20, ci 17, rules 11, source 6, policy 3, run 3, sep1 3, cli 2, report 2, strkey 2, threshold 2, model 1, sanitize 1. The dependency-free research tasks (0.5, 0.8, 0.9, 0.12, 0.13) are labelled `good first issue` and `help wanted`: they need public documentation and writing, not network access or a decision from the owner.

## 6. Sources

- `https://docs.drips.network/wave/` (program description, monthly cadence, January 2026 Stellar launch)
- `https://docs.drips.network/wave/maintainers/participating-in-a-wave/` (app installation, applying a repository, Trivial, Medium and High tiers, marking issues Resolved before the wave ends)
- `https://docs.drips.network/wave/maintainers/faq/` (Add to Wave, GitHub-added issues defaulting to Trivial at 100 points, appeal path, phone verification, one to two hours scoping good first issues, daily dashboard checks)
- `https://docs.drips.network/wave/maintainers/repo-application-limits/` (per-user and per-org caps, rejected repos cannot be re-applied, three appeals)
- `https://docs.drips.network/wave/maintainers/points-budgets/` (Trivial at 100 points, per-project and per-org budget ceilings, audits triggered by adding issues or raising tiers)
- `https://www.drips.network/wave/stellar` and `https://www.drips.network/wave/stellar/repos` (ten waves concluded, no active or upcoming wave, budgets, 839 approved repositories on 2026-10-30 and 838 on 2026-11-02)
- `https://nansen.ai/post/stellar-q2-2026-report` (Q2 2026 network figures quoted in section 5)

## 7. The organization has a second repository now

`posture-sec/airlock` went public on 2026-11-02. It is the sibling scanner: a Python 3.11 CLI that audits Stellar and Soroban *repositories* for supply-chain compromise and release-integrity gaps, where Gauge audits *account configuration* on-chain. Same owner, same conventions, no shared code, and it arrived by recovery rather than by plan: the account that owned it (`airlock-sec`) was suspended, so the repository, its history and its documentation were moved here and re-pointed, which is task 1.15 in its own roadmap.

State read on 2026-11-02, from that repository's files and the live GitHub API rather than from this one:

| | Airlock |
|---|---|
| Roadmap | 45 of 95 tasks across Phases 0 to 8, 47%; Phase 1 is 12 of 15; its own gates row reads 0 of 10 |
| Rules and detectors | 22 rules in four rule files, seven detector modules |
| Tests | 709 passed, 1 skipped, run locally today; `ruff`, `mypy --strict` and its self-scan all clean, self-scan exit 0 |
| Public surfaces | docs site live at `https://posture-sec.github.io/airlock/`, `v0.0.1` cut as a pre-release, 23 labels, 12 topics, `protect main` ruleset requiring its two real checks, direct push observed refused |
| Wave work drafted | fifteen scoped issues written under `docs/wave/`, deliberately not published, plus its own `docs/wave/application.md` and settings runbook |
| Known blemish | 46 of the 78 commits on `main` are authored under the two identities of the account that was suspended (`Gabriel Onwachie <121898101+Gabbydunkk@…>` and `gabbydunkk <…@gmail.com>`), kept rather than rewritten so the merge records its changelog cites stay intact; the pre-release notes say so |

What changes here is not Gauge's readiness but the shape of the decision:

1. One Drips Wave App installation at org scope covers both repositories, so the install and phone-verification steps happen once.
2. The per-org cap and the per-org budget ceiling are shared. Nothing in the maintainer docs says how large either is, so the cost of applying both in one cycle is unknown, and a rejection is not refundable: the docs say a rejected repository cannot be re-applied and appeals are capped at three with waiting periods between them.
3. Airlock is the more complete submission today, on every count in that table, and its fifteen issues are already scoped with acceptance criteria. Gauge has the better story to demonstrate (a live read of a real account, a docs site, a Scorecard badge) but is thinner on rules until the signer family lands.
4. That ordering is not a reason to submit Airlock first anyway. Its own runbook puts the Drips application *before* publishing the issues, because on an unapproved repository the `Stellar Wave` label is stripped silently and the issues score nothing; its gate list in `docs/wave/application.md` is what decides, not this file.

The practical consequence for this packet: read the caps on the dashboard first, and if the cap allows one repository per cycle, spend that one on whichever of the two has its gate list clear on the day the window opens. If it allows two, they still want different cycles, because a maintainer has to review contributor pull requests during the active week and one person running two waves at once is how issues end up unresolved at the deadline, which costs the contributor their points rather than costing the organization anything.
