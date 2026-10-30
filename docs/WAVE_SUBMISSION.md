# Drips Wave submission packet: Gauge

Working notes for applying `posture-sec/guage` to a Stellar Wave cycle. Program facts below were read from the sources listed in section 6 on 2026-10-30, not recalled. Nothing in this file claims the repository is approved; it is not.

## 1. Where the program stands

- Drips Wave launched with the Stellar ecosystem in January 2026, with waves typically running one week per month.
- As of 2026-10-30 the Stellar Wave page shows ten waves completed and states there are no active or upcoming waves at the moment. A recent cycle window in the page data ran from 2026-10-15T13:00:46Z to 2026-10-22T13:00:56Z. Recent budgets shown are $75,000, with earlier waves at $60,000.
- The same page listed 839 repositories approved for the Stellar Wave program on 2026-10-30. The sample on the first page includes early-stage repos alongside funded protocols, so an unbuilt product is not automatically out of scope.
- Not verified: whether this repository is already on that list. The only reliable check is the Maintainers dashboard after signing in with GitHub.

## 2. Why this repository is not applying yet

Two reasons, ordered by how long each takes to clear.

1. No cycle is open, so there is nothing to apply into today.
2. Nothing audits anything yet. `go run ./cmd/stellar-gauge` prints a scaffold notice and exits 3. A submission with no working check reads as unfinished, and the downside is asymmetric: a rejected repository cannot simply be re-applied, appeals go through the dashboard, are capped at three, and carry waiting periods. Spending the first application on a scaffold trades a permanent strike for nothing.

## 3. Gate list before clicking Apply

- [x] PR #2 merged, so `main` carries CI, the roadmap gate, `README.md`, `CONTRIBUTING.md`, `SECURITY.md`, `CODEOWNERS`, issue forms and the banner. (merged 2026-10-30 as `ae6a05d`; the first push-event CI run on `main` passed)
- [x] Task 1.3 closed by observing the direct-push refusal against `main` itself, not a probe branch. (refused 2026-10-30 with "Changes must be made through a pull request", `main` unchanged)
- [x] Task 1.7 done: a Scorecard run where Branch-Protection returns a score. (2026-10-30: score 4, overall 6.2, Scorecard v5.6.0, badge live in the README. The three warnings about required approvals are accepted under D-14 because the repository has one maintainer, and they will keep the score down until a second reviewer exists.)
- [ ] Task 2.9 done, so `stellar-gauge check` reads a real account and prints a report. This is the minimum credible demo.
- [ ] A short screen recording of that run end to end, which the playbook treats as the difference between a submission and an unfinished one.
- [ ] Task 0.4 answered with named operators, since the pitch rests on people who would run this on a schedule.
- [ ] Task 4.2 at least partly done: a rule reference a contributor can work from without reading Go source.
- [ ] The per-user and per-org repository caps for the cycle checked before deciding what to submit.

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
- `https://www.drips.network/wave/stellar` and `https://www.drips.network/wave/stellar/repos` (ten waves completed, no active or upcoming wave, budgets, 839 approved repositories)
- `https://nansen.ai/post/stellar-q2-2026-report` (Q2 2026 network figures quoted in section 5)
