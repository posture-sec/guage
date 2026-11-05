#!/usr/bin/env bash
# Builds the planned GitHub issue backlog from the open tasks in ROADMAP.md.
#
# One curated row per task ID supplies the title, type and area labels. The body
# is read from ROADMAP.md at run time, so acceptance criteria cannot drift away
# from the plan. A hidden marker line in each body keeps the backlog idempotent:
# rerunning the script only creates tasks that have no issue yet.
#
# Bodies follow PI-1, the organization's issue contract, published in
# posture-sec/.github's ISSUE_CONTRACT.md: the marker, a one-sentence brief, and
# nine numbered sections in a fixed order. Three of those sections cannot be
# derived from a roadmap row, so they are curated per task in issue_prose below:
# the kind, the brief, the why paragraph and the out-of-scope sentence. A task
# with no prose row gets no body. That is deliberate. The contract says a section
# that restates the title is how an empty issue gets written, so the script
# refuses instead of inventing one. A prose row belongs to an open task: when the
# row ticks, delete its prose with it, because a paragraph written about work that
# is now finished reads as a claim about the tree rather than as history.
#
# The rest is derived: paths from the area label, the rules that bind a change
# from AGENTS.md section 4, the commands from section 5, and the split between
# what a desk can prove and what needs a runner from the kind.
#
# Usage: scripts/generate_issues.sh [options] [command]
#   --phase N       limit to one phase (0 to 6, or "gates")
#   --roadmap FILE  roadmap to read (default ROADMAP.md)
#   --repo OWNER/NAME  target repository (default posture-sec/guage)
#   --apply         create the missing issues with gh
#   --update        also rewrite the body and labels of issues that exist
#   --check         fail if an open task has no row, no prose, or a row has no task
#   --body-for ID   print the PI-1 body for one task to stdout and exit
#   --include-done  also emit rows for tasks already ticked
set -euo pipefail

repo="${GUAGE_REPO:-posture-sec/guage}"
roadmap="ROADMAP.md"
phase=""
apply=0
update=0
check=0
include_done=0
body_for=""

while [ $# -gt 0 ]; do
  case "$1" in
    --phase) phase="${2:?--phase needs a value}"; shift ;;
    --roadmap) roadmap="${2:?--roadmap needs a value}"; shift ;;
    --repo) repo="${2:?--repo needs a value}"; shift ;;
    --apply) apply=1 ;;
    --update) update=1 ;;
    --check) check=1 ;;
    --body-for) body_for="${2:?--body-for needs a task id}"; shift ;;
    --include-done) include_done=1 ;;
    *) echo "generate_issues: unknown argument ${1}" >&2; exit 2 ;;
  esac
  shift
done

if [ ! -f "${roadmap}" ]; then
  echo "generate_issues: ${roadmap} not found" >&2
  exit 1
fi

repo_name="${repo##*/}"

# id, type, area, title. Titles stay under 72 characters and use the
# Conventional Commits prefix that matches the type label.
backlog() {
  printf '%s\n' \
    '0.2	type: research	area: docs	docs(decisions): record name, org and license choices' \
    '0.3	type: research	area: docs	docs(decisions): choose code sequencing against Airlock v0.1.0' \
    '0.4	type: research	area: docs	docs(research): validate demand with three real operators' \
    '0.5	type: research	area: docs	docs(research): survey existing Stellar self-audit tools' \
    '0.6	type: research	area: source	docs(research): capture Horizon account JSON per scenario' \
    '0.7	type: research	area: threshold	docs(research): verify threshold semantics on testnet' \
    '0.8	type: research	area: sep1	docs(research): pin SEP-1 and SEP-29 field requirements' \
    '0.9	type: research	area: source	docs(research): check Horizon status and RPC account coverage' \
    '0.10	type: research	area: strkey	docs(decisions): confirm own strkey package over the SDK' \
    '0.11	type: research	area: docs	docs(research): run the name collision check for the CLI' \
    '0.12	type: research	area: source	docs(research): measure endpoint limits, pick defaults' \
    '0.13	type: research	area: policy	docs(research): review the TOML dependency before adopting' \
    '1.2	type: security	area: docs	chore(security): confirm account and org hardening' \
    '1.3	type: security	area: ci	chore(security): protect main and add CODEOWNERS' \
    '1.5	type: tooling	area: ci	ci: run gofmt, vet, staticcheck, race tests, govulncheck' \
    '1.6	type: tooling	area: ci	ci: enable Dependabot for gomod and Actions' \
    '1.7	type: tooling	area: ci	ci: add a Scorecard workflow that reads branch protection' \
    '1.8	type: docs	area: docs	docs: add SECURITY, CONTRIBUTING and the PR template' \
    '1.9	type: tooling	area: ci	ci: enforce the roadmap rule with a required gate check' \
    '1.10	type: docs	area: docs	docs(readme): publish the skeleton with status and badges' \
    '1.11	type: docs	area: docs	docs: keep the briefing doc set in the repository root' \
    '1.13	type: tooling	area: ci	ci: finish program-submission repo hygiene' \
    '1.14	type: tooling	area: ci	ci: keep the issue backlog in step with the roadmap' \
    '1.15	type: docs	area: docs	docs(wave): keep the submission packet current' \
    '2.1	type: feature	area: model	feat(model): complete types, severity ordering, constructors' \
    '2.2	type: feature	area: strkey	feat(strkey): classify keys, verify checksums, detect seeds' \
    '2.3	type: feature	area: threshold	feat(threshold): Reachable and MinSigners with cross-check' \
    '2.4	type: feature	area: policy	feat(policy): strict config decode and input validation' \
    '2.5	type: feature	area: sanitize	feat(sanitize): strip control, bidi, zero-width characters' \
    '2.6	type: feature	area: report	feat(report): deterministic renderers and exit mapping' \
    '2.7	type: feature	area: source	feat(source): Horizon client with caps and bounded retries' \
    '2.8	type: feature	area: run	feat(run): orchestrate policy, source and report' \
    '2.9	type: feature	area: cli	feat(cli): wire check, audit and version subcommands' \
    '2.10	type: test	area: run	test: prove every fail-closed path exits non-zero' \
    '2.11	type: feature	area: source	feat(source): model and decode signed-payload signers' \
    '3.1	type: feature	area: rules	feat(rules): signer and threshold rules with fixtures' \
    '3.2	type: feature	area: rules	feat(rules): flag drift and undeclared clawback rules' \
    '3.3	type: feature	area: rules	feat(rules): missing account and network mismatch rules' \
    '3.4	type: feature	area: rules	feat(rules): role profiles and accepted-risk handling' \
    '3.5	type: feature	area: sep1	feat(sep1): guarded stellar.toml fetch, identity rules' \
    '3.6	type: test	area: rules	test(fixtures): golden outputs for every scenario' \
    '3.7	type: feature	area: policy	feat(policy): zero-config facts report and example config' \
    '4.1	type: research	area: rules	docs(dogfood): audit real accounts with zero false HIGH' \
    '4.2	type: docs	area: docs	docs(usage): install, check, audit and rule reference' \
    '4.3	type: docs	area: docs	docs(readme): complete with limits wording and non-checks' \
    '4.4	type: tooling	area: ci	ci: build and test on Linux, macOS and Windows' \
    '4.5	type: tooling	area: ci	ci: release workflow with build provenance attestation' \
    '4.6	type: security	area: ci	chore(security): reach a Scorecard score of 8 or higher' \
    '4.7	type: tooling	area: ci	ci: run Airlock as a scanner in this repository' \
    '4.8	type: tooling	area: ci	ci(release): tag v0.1.0 with checksums and verification' \
    '4.9	type: docs	area: docs	docs(announce): publish measured results from real audits' \
    '5.1	type: feature	area: source	feat(source): multi-endpoint comparison, src-disagreement' \
    '5.2	type: feature	area: run	feat(run): snapshot and diff against stored baselines' \
    '5.3	type: feature	area: report	feat(report): markdown output with escaped evidence' \
    '5.4	type: tooling	area: ci	ci: scheduled audit template that opens issues on findings' \
    '5.5	type: feature	area: rules	feat(rules): trustline exposure facts for held assets' \
    '5.6	type: feature	area: sep1	feat(sep1): SEP-29 memo-required check for merchants' \
    '5.7	type: test	area: source	test(perf): rate-limit and many-account load checks' \
    '5.8	type: docs	area: docs	docs(site): documentation site separate from the README' \
    '6.1	type: docs	area: docs	docs: align every document with v0.2 behavior' \
    '6.2	type: tooling	area: ci	ci(release): tag v0.2.0 with attestation' \
    '6.3	type: research	area: rules	docs(triage): review external false-positive reports' \
    '6.4	type: research	area: docs	docs(decisions): decide funding and applications (D-07)' \
    '6.5	type: docs	area: docs	docs(outreach): contact issuers, anchors and treasury teams' \
    'G1	type: test	area: rules	gate: every P0 requirement has passing tests' \
    'G2	type: test	area: rules	gate: every scenario has a fixture and expected findings' \
    'G3	type: test	area: cli	gate: fail-closed paths all exit non-zero' \
    'G4	type: research	area: rules	gate: dogfood shows zero false HIGH findings' \
    'G5	type: security	area: ci	gate: checks clean and Scorecard at 8 or higher' \
    'G6	type: tooling	area: ci	gate: releases attested, verification tested' \
    'G7	type: docs	area: docs	gate: every document agrees with the code' \
    'G8	type: security	area: ci	gate: required checks set, direct push to main rejected' \
    'G9	type: research	area: docs	gate: no open High risk without mitigation' \
    'G10	type: docs	area: docs	gate: ROADMAP.md current in the latest merged PR'
}

# Per-task prose PI-1 cannot derive: id, kind, brief, why, out-of-scope, one tab
# separated line each. kind is task, gate or owner, matching the contract's
# marker vocabulary: a gate closes on an observation, an owner row is an act only
# the account owner can take, and neither should read like a pull request.
# The heredoc is quoted so prose may hold apostrophes and backticks.
issue_prose() {
  cat <<'PROSE'
0.4	owner	Three named operators say they would run this on a schedule, and their answers are in the changelog.	Every rule in this repository encodes a guess about what an issuer, a merchant or a treasury holder cares about, and nothing has tested that guess against a person. The demand check is the row that asks three real operators whether they would run a read-only posture report weekly, what they would ignore and what they would act on. The Accensa team is the contact this project already has; a merchant and an issuer or treasury holder are the two it does not. Fewer than three yes answers is a finding rather than a failure, and the row says to stop and revisit scope if that is the answer, which makes this the owner's act: both the introductions and the scope decision run through him.	Not a survey tool, not a landing page, not a doctored sample report to show someone. A form or a public post does not count as an operator answering; the row wants three named humans and their words.
1.2	owner	The account and the organization have hardware-key or passkey 2FA, a protected recovery email and a minimal owner list.	Everything this repository protects is public except the credentials that can rewrite it. The Scorecard job reads repository settings, which is why 1.7 reports on branch protection and says nothing about whether the account administering those settings can be taken over with a password, and the program application is filed as that same account. No CI job here can observe any of it, which is exactly what the row means by checked off by the owner, not verifiable by CI. The row stays open until the four checks are recorded here.	Repository settings, CODEOWNERS and required checks are 1.3 and G8, both separate rows. Nothing in this one changes a file, and a pull request that claims to close it has misunderstood it.
1.15	task	The submission packet says what is true in the week someone reads it.	docs/WAVE_SUBMISSION.md is the document a program maintainer reads before deciding whether this organization's repositories are worth pointing people at, and it is the one most likely to rot, because its numbers describe a moment rather than a system. That already happened once: task 1.22, on 2026-11-03, found section 5 still carrying the backlog as it had been drafted, with counts matching neither ROADMAP.md nor GitHub. D-13 says do not apply until the gate list clears and D-10 published the backlog before any application, so this file is where those two decisions have to agree every week, with the check date written at the top.	Filing the application is the owner's act. Restating the packet from an older copy is the failure mode this row exists to prevent, so re-read each fact from the file or the API it came from and write the date it was read. No tiers, scores or promises of our own invention.
4.1	task	Real operator accounts get audited, expectations get declared, and every HIGH that came back is explained or fixed.	Sixteen rules are registered, and each was written against a shape this repository or posture-sec/stellar-account-fixtures could read. That proves the decoder and the rules. It does not prove the rules are the ones an operator would call correct, which is the difference between a tool that reports posture and a tool that cries. Task 4.1 is the run and gate G4 is the reading: real accounts, declared expectations, zero false HIGH findings. The thirty-eight files under testdata/golden are exactly the kind of confidence this row distrusts, because a golden pins what the code said rather than what was right.	Not new rules, and not a softened severity to make a false positive disappear. If a real account produces a HIGH that holds up under review, the finding is the row's answer. Not a private run either: the accounts and the readings have to be written down.
4.2	task	Every rule has an article a person can read without opening a Go file.	site/rules.html writes out all sixteen rules, each with its condition, what the finding names, whether an acceptance can hide it and what a declared role does to the severity, so the rule-reference half landed on 2026-11-04. What the row still withholds is the other half: the install, check, audit and config articles were last re-read against a surface of seven flags across two subcommands, and 3.7 merged on 2026-11-05 with a facts block that prints ahead of the findings and a check that is a full report with no policy file. Those pages now describe output the tool no longer produces, and the row ticks when they describe the one it does.	Not a new page and not a rule severity softened to make a complaint go away. Do not document a flag that does not exist, and do not tick the row from the 2026-11-04 readings: the surface moved on 2026-11-05.
4.3	task	The README says plainly what Gauge does not look at.	A read-only posture tool invites two wrong readings: that a clean report means an account cannot be misused, and that the absence of a finding means the absence of a risk. The report footer carries the limits wording for the second one, but the person deciding whether to run the tool reads README.md rather than a footer. This row is the explicit what this does not check section: transactions, custody of keys, what an account has signed, the assets themselves, and the shapes no fixture can reach because they would need a submission the project forbids.	Not a marketing rewrite and not a longer feature list. Do not soften a claim to make the gaps read smaller; the gaps are the point of the section.
4.4	task	The suite is green on Linux, macOS and Windows on a runner rather than on three desks.	The CLI is aimed at operators who run it in CI and on laptops, and the cross-platform cost is mostly paths, line endings and the two places the code touches a filesystem: the report writers and the baseline files 5.2 will add. Today the job in .github/workflows/ci.yml runs a single Linux image, so nothing here has been observed on Windows, which is where a path or newline assumption surfaces. The row is small and it is the kind of change that pays for itself the first time it fails.	Do not add a dependency or a build tag to make a platform pass, and do not disable a test on one OS to get the matrix green. That is weakening a check, and AGENTS.md section 4 refuses it.
4.5	task	A release artifact carries provenance, and the command that verifies it has been run and pasted.	D-12 exists because tagging the scaffold would have advertised a release that audits nothing, so the release work is deliberately late. When it lands it has to be the attested kind: a build provenance statement someone can check, and a verification command in docs/RELEASE.md that somebody has actually run. A release with no verification is how an unpinned download becomes the supply chain, which is the failure 1.6, 1.7 and this row together exist to avoid. The tag itself is the owner's act, recorded under 4.8.	Publishing the tag is not this row. Nothing here authorizes bumping a pinned action SHA without reading its diff, and no new module enters go.mod on the way to a release.
4.6	task	The Scorecard number is at or above 8, with the run that printed it named.	scorecard.yml has run since 1.7 and the badge in README.md carries whatever it printed, so the score is visible rather than asserted. Reaching 8 is mostly the two halves this row names: keeping govulncheck clean, and closing what the tool reports about pinned dependencies, dependency update tooling, SAST and fuzzing. D-14 records the owner's decision to accept three Branch-Protection warnings while the repository has one maintainer, and that decision caps what a pull request can do here: part of the gap is an organization setting, not a diff.	Do not mute a finding to raise the score, and do not add a scanner for its own sake. The checks in AGENTS.md section 5 are the set; expanding it is its own roadmap row.
4.7	task	Airlock runs as a scanner in this repository's CI, pinned by SHA and passing.	The sibling scanner reads repositories and Gauge reads accounts, so running Airlock here is the cheapest proof that two tools in one organization agree about the same organization. The row is written down rather than done because of an addressable dependency: posture-sec/airlock task 5.5 is still open and its only tag is a v0.0.1 pre-release. Installing a pre-release scanner into a required check would make this gate depend on an unpinned, unsigned third-party build, which is the shape task 5.3 exists to remove.	Do not vendor Airlock's source to get a scan running, do not pin to a branch rather than a SHA, and do not turn a required check into an advisory one because a pre-release is noisy. Confirm the scanner is still published before starting: the owner reported its account suspended in late 2026, and no document in this repository records that.
4.8	owner	v0.1.0 exists as a tag and a release with checksums, and the gates have been reviewed for it.	D-12 exists because tagging the scaffold would have advertised a release that audits nothing, so this row is the moment the repository stops describing itself as a scaffold. Tagging, publishing checksums and reviewing G1 to G10 for the release are acts on this organization's repository rather than edits inside it, which is why the issue is a record and not a task for someone else. Its dependencies, 4.1 to 4.6, are the substance: a dogfooded rule set, finished docs, a cross-platform matrix, an attested release path and a recorded Scorecard reading.	Do not open a pull request to create the tag. Nothing here authorizes a pinned action bump or a new module on the way to a release.
4.9	owner	The announcement carries counts from real runs rather than the plan.	The row's wording is deliberate: announce with measured results, accounts audited and issues found, not intentions. A public post from this organization is made with its owner's accounts and stands as its record afterwards, and the numbers in it have to come from 4.1's dogfood rather than from a roadmap. This issue exists so the announcement stays traceable to the run behind it instead of to a draft.	Not a campaign, not a scheduled series of posts, and no claim about an account beyond what a report shows. A post describes posture and nothing stronger.
5.1	task	Two endpoints that disagree produce src-disagreement instead of a quiet answer.	Everything Gauge says about an account comes from one read of one endpoint. FR-14 asks for multi-endpoint comparison behind the Source interface, so that an instance serving stale or filtered data shows up as a finding rather than as a clean report. The dependency is 4.8: this is v0.2 work, and the interface it extends is the one D-05 kept narrow so a second implementation would stay honest. The rules remain pure and only the source layer grows.	Not the Stellar RPC implementation, which is row 7.1 in the backlog, and not a change to how any existing rule decides severity. A second endpoint is not a second opinion the report may average: disagreement gets reported, never resolved quietly.
5.2	task	A baseline can be saved, diffed, and read as a change in posture rather than a fresh report.	FR-15 asks for snapshot and diff against a saved baseline, which is what turns this from a question asked every week into a record that can be reviewed: an operator who will not read a full report will read one line saying a signer appeared. The scaffold already reserves internal/run/snapshot.go, internal/run/diff.go, internal/run/compare.go and the two commands in cmd/stellar-gauge, each one line long today. This row also introduces the only file write the project permits, so read the AGENTS.md section 4 note on --out first: that section cites FR-13 for the snapshot flag while PRD.md puts snapshot and diff at FR-15, and two documents disagreeing is a stop-and-ask rather than a detail to settle quietly.	Baselines live outside the audited repository and the guidance says so. A snapshot is not a secret store and never holds a seed. No daemon, no history database, no alert integration, and no new dependency to serialize one.
5.3	task	A markdown report renders chain-derived text as the text it is.	FR-16 asks for markdown output, and every string in that report comes from a chain read: an account name, a data entry value, a home domain. internal/sanitize already strips control, bidi and zero-width characters for the text and JSON renderers, and internal/report/markdown.go is a one-line stub. The new surface is markup: a signer label carrying a link, an image or a script tag has to survive into fenced evidence as literal text, which is why internal/sanitize/markdown.go already sits beside the rest of that package.	Not a template system, not HTML output, not a general renderer. Do not widen the sanitizer's contract for convenience; add markdown cases as fixtures with the hostile bytes written from fragments, the way AGENTS.md section 4 requires.
5.4	task	A scheduled run opens an issue only when something moved, with minimal permissions and SHA pins.	This row already became a separate repository. posture-sec/stellar-posture-watch is a composite GitHub Action that runs gauge audit on a schedule and keeps one tracking issue current; it builds Gauge at a pinned SHA and maps the four exit codes, so the template this row asks for exists in a working form and should be written against what that repository learned rather than invented here. The row stays open because Gauge's documentation has no scheduled-audit guidance until it points at the action.	Do not rebuild the action inside this repository and do not add a service or a daemon here. Minimal permissions and SHA pinning are conditions rather than preferences. A body a schedule writes is exempt from the organization's issue contract, which the action's own CONTRIBUTING.md states.
5.5	task	The report says which assets an account holds and what each issuer can do to them.	A trustline is an agreement with an issuer, and the flags on that issuer's account decide what can happen to the balance: auth_required, auth_immutable, auth_clawback_enabled. Today Gauge reports the account it was pointed at and nothing about the counterparties behind the assets it holds, which leaves out the exposure a treasury holder most wants measured. This is facts before it is findings. testdata/horizon/README.md records that scenario 8's auth_revocable, auth_immutable and auth_clawback_enabled halves stay uncaptured in both sets, so some shapes here will be built rather than observed and have to be labelled that way.	No balance values in a report body, and no new HTTP call outside internal/source's guarded client. Do not convert a facts line into a severity claim the fixtures cannot support.
5.6	task	A merchant account that can enforce the memo rule gets warned about it.	SEP-29 is the requirement that an exchange or merchant account set auth_required so a sender must supply a memo, and a missing memo is how deposits get lost. The check is a flag read plus a domain question, which is why 0.8 is the dependency: that row pinned the SEP-1 and SEP-29 field requirements. internal/rules/identity.go already consumes a parsed stellar.toml through the guarded fetcher 3.5 shipped, so this is a rule beside those rather than a new mechanism.	Rules stay pure functions: the answer comes from the snapshot and the caller-supplied sep1 result, never from a fetch inside the rule. No transaction building, so nothing here can prove a memo end to end, and the docs have to say that.
5.7	task	The rate limits and concurrency have measured numbers behind them instead of a comment.	internal/source implements timeouts, body size caps, bounded retries and bounded concurrency, and the fake-server tests in internal/source/horizon_test.go prove each failure mode one request at a time. What no test shows today is the shape an operator actually hits: many accounts, one rate-limited endpoint, and whether the bounded concurrency AGENTS.md section 4 requires still holds there. This row is that measurement, and it is the one that says which of the defaults 0.12 picked are right rather than leaving them asserted.	Not a load-testing framework, and no real-network default: the test stays out of CI unless tagged livenet, per AGENTS.md section 4. Do not raise a cap to make a test pass; the caps are the point.
5.8	task	The documentation site covers what a persona needs, not only what the CLI ships.	The site at posture-sec.github.io/guage is live under site/ with the pages 5.9 shipped and both palettes from 5.10, and 4.2 filled in the rule reference. What this row still asks for is persona depth: end-user guides per operator kind, a developer guide, and a contributing path, none of which can be written honestly while the shipped surface is seven flags across two subcommands. Its dependencies 4.2 and 4.3 are both open, and its Done when clause requires every page to reflect shipped behavior, so a page written now is a page to unwrite later.	The economic-model and contract-reference clauses in the original row describe a Soroban project rather than this CLI; they were corrected when 5.9 shipped and stay out of scope. Not a new site generator: the pages are static HTML deployed by .github/workflows/pages.yml.
6.1	task	Every root document describes the tool that exists after Phase 5, in one pass.	Gauge keeps seven briefing documents in the repository root, and project_structure.md section 7 gives each one an update trigger, which means drift is possible by construction rather than by accident. This row is the sweep that closes gate G7 for v0.2: PRD.md, ARCHITECTURE.md, ARCHITECTURE_ESSENTIALS.md, project_structure.md, AGENTS.md, CLAUDE.md and README.md re-read against the code as it stands after Phase 5. It is deliberately one pull request rather than seven, because the interesting contradictions are the ones between documents.	No behavior changes. If a document is wrong about the code, the code stays and the sentence moves. ARCHITECTURE_ESSENTIALS.md has a stated ceiling of about 150 lines, so growth there needs restructuring rather than appending.
6.3	task	Each false-positive report from outside the organization becomes a rule change, an example, or a written reason.	A tool that reports posture is judged by the findings people disagree with, and disagreement from someone running it on their own account is the most useful signal available after release. This row keeps that intake from turning into ad hoc edits: each report gets reviewed, and the answer is either a rule change with fixtures on both sides, a documented example showing why the finding is correct, or a recorded decision saying the reporter was right.	The rules are pure functions in this repository, so this is not a plugin system, not user-authored rules and not a per-account severity dial. An accepted risk stays an [[accept]] entry in a policy file, and only the owner adds those.
6.2	task	v0.2.0 is tagged with the attestation, after the documents say what the code does.	Phase 5's features are the content of v0.2, and 6.1 is the sweep that makes the documentation describe them, which is why the row depends on 6.1 and why gate G7 hangs behind it. The tag and the release are the owner's act, following the attested path 4.5 built rather than inventing a second one.	No new feature enters between 6.1 and this tag; a feature discovered late is a roadmap row for the next release. The verification command comes from 4.5 and stays one documented path.
6.4	owner	D-07 is decided: this project pursues funding and programs, or it does not, with the adoption evidence named.	The decision log has carried D-07 as Open since the bootstrap, with the note that it comes after v0.2.0 and adoption evidence. Two other decisions lean on it: D-13 says do not apply to a Drips Wave until the gate list clears, and D-10 published the backlog before any application. This row is where that choice gets made and written down with the evidence behind it, because the alternative is a set of repository changes made for a program nobody has agreed to join.	Not the application itself, which is filed through a console that authenticates as the owner's account. Not a new gate list either: D-13's is the current one until this decision changes it.
6.5	owner	Issuers, anchors and treasury operators have been contacted, and what they said is in the record.	Outreach is the same evidence 0.4 asked for, after release: the operators this tool is for, reached with a report they can read. It is the owner's act because it happens under his name and through his contacts, and it feeds D-07, which wants adoption evidence before this organization decides whether to pursue programs and funding.	Not a mailing list, not a CRM, not an automated campaign. Nothing collected here is stored in this repository; the changelog carries the summary and the date it was written.
G4	gate	Zero false HIGH findings on real operator accounts, once expectations are declared.	The claim is about the world rather than the tree: an operator runs Gauge against accounts whose configuration is known, declares what is expected, and no HIGH comes back that was not actually there. Task 4.1 produces the run and this gate is the reading that has to hold afterwards. Sixteen rules and thirty-eight golden files prove internal consistency; only a live account with a person checking the answers proves calibration.	A gate: nobody closes it with a diff. Not a severity downgrade to reach zero, and not a self-report from the person who wrote the rule without naming the accounts and the readings.
G5	gate	The three Go checks are clean on a runner and the Scorecard number is at or above 8.	govulncheck, staticcheck and go vet are the third-party reading of this repository's hygiene, and CI runs them as the ci / go and ci / vuln jobs, so the first half of this gate is visible from a run log. The Scorecard half depends on organization settings that D-14 records as accepted for now, with three Branch-Protection warnings outstanding while the repository has one maintainer. The gate therefore has a hosted half no contributor can produce from a fork, and 4.6 is the row that moves what a pull request can move.	A gate: it closes on a recorded run, not a claim. Do not add a skip, an exclusion or a config loosening to make a tool go quiet; that is the exact weakening AGENTS.md section 4 forbids.
G6	gate	A release exists with a provenance attestation and the verification command has been run.	This is 4.5's evidence seen from the other side: an attested artifact plus a documented verification someone executed. D-12 kept the tag off the scaffold precisely so that a release would mean an auditing build, so the gate is not satisfied by a workflow file alone. The observation is a run id and the output of the verification command, dated in UTC.	A gate: no pull request closes it. Not a release cadence, not publish-on-merge automation, and not a checksum list with no command that verifies it.
G7	gate	The seven root documents and the site agree with the code in one recorded pass.	The documents are the interface for agents and contributors, and each has its own update trigger in project_structure.md section 7, so agreement is something to observe rather than assume. This repository has already been corrected twice by its own documents: a line in ARCHITECTURE.md 11 that claimed Airlock ran in this CI when it did not, and an AGENTS.md section 5 note that outlived the fuzz code it described. G7 is the sweep that finds the current set of those, with 6.1 doing the v0.2 pass.	A gate: it closes on a read documented per document. Not a licence to expand the documents; a sentence that needs more than a line belongs in the ARCHITECTURE.md section its essentials item cites.
G8	gate	A direct push to the default branch is rejected and the required checks are the two this repository depends on.	1.3 added CODEOWNERS and protection, and 1.9 made the roadmap rule a required check, so the machinery is here. The gate asks for the observation instead: a push attempt that GitHub refused, and a branch setting listing ci / go and roadmap-check / gate as required. D-14 records the owner's decision to accept the no-approver warnings while this is a one-maintainer repository, so what this gate wants is the state rather than more protection than the organization can support.	A gate, and a hosted one: repository settings are the owner's to change and to read. A pull request cannot close it, and a ruleset added in a fork proves nothing about this repository.
G9	gate	Every High-rated risk in the register has a mitigation task or an accepted decision beside it.	ROADMAP.md section 8 rates the risks this project lives with, and the failure mode is not a bad risk but a stale one: a High entry nobody has looked at since it was written, whose mitigation shipped or evaporated. The gate is the pass that re-reads each High row against the task list and the decision log and marks either a task id or a D-number. D-13 and D-14 are examples of risks answered by a decision rather than by code.	A gate: it closes on a written re-read. Not a new risk framework and not a severity debate for its own sake; if a risk needs a mitigation, the fix is a roadmap row, and the row comes first.
G10	gate	ROADMAP.md is current in the merge that most recently touched it.	The rule that every contribution updates the roadmap is enforced twice, by the roadmap-check / gate job and by the pull request template's checkbox, and this gate is the human half: the file's Last updated line and its progress table describe the state the merged tree actually has. The trap is a row ticked without the table regenerated, or a changelog date invented rather than read from date -u, and scripts/roadmap_progress.sh --check is what makes that arithmetic visible instead of promised.	A gate: it closes on an inspection of the newest merged change, naming the run or the commit. Not a licence to rewrite history or to age the file for tidiness.
PROSE
}

strip_backticks() {
  tr -d '`'
}

# Returns the roadmap line for a task ID, or nothing when absent.
roadmap_line() {
  grep -m1 -E "^- \[[ x]\] \*\*${1}\*\*" "${roadmap}" || true
}

verb_for() {
  case "$1" in
    "type: feature") printf 'feat' ;;
    "type: test") printf 'test' ;;
    "type: tooling") printf 'ci' ;;
    "type: security") printf 'chore' ;;
    *) printf 'docs' ;;
  esac
}

# The three tiers match the Drips Wave complexity levels, so the `complexity:`
# label and the value the maintainer enters in the wave dashboard agree:
# docs.drips.network/wave says a maintainer assigns Trivial, Medium or High per
# issue, and that issues tagged from GitHub instead land in Trivial at 100 points
# regardless of the work. This script therefore never adds the `Stellar Wave`
# label: it would enroll the issue at the wrong value. The `wave:` field in the
# marker records the shape of the work for the dashboard entry. Roadmap sizes are
# unmeasured guesses, so the mapping is one way: S to Trivial, M to Medium, L and
# XL to High.
complexity_for() {
  case "$1" in
    L | XL) printf 'complexity: high' ;;
    M) printf 'complexity: medium' ;;
    S) printf 'complexity: trivial' ;;
    *) printf '' ;;
  esac
}

tier_for() {
  case "$1" in
    L | XL) printf 'High' ;;
    M) printf 'Medium' ;;
    S) printf 'Trivial' ;;
    *) printf '' ;;
  esac
}

# Wraps one paragraph per input line to 96 columns. The prefix stays on the first
# line and continuation lines get the same width of spaces, so markdown keeps a
# list item or a quotation in one piece. A line whose longest word already exceeds
# the width passes through: a path or a command is not worth breaking.
wrap() {
  awk -v pref="${1:-}" -v ind="${2:-}" '
    BEGIN {
      if (ind == "") { ind = pref; gsub(/[^ ]/, " ", ind) }
    }
    {
      s = $0
      gsub(/[ \t]+/, " ", s)
      sub(/^ /, "", s)
      sub(/ $/, "", s)
      if (s == "") { print pref; next }
      n = split(s, w, " ")
      line = pref; has = 0
      for (i = 1; i <= n; i++) {
        cand = (has ? line " " w[i] : line w[i])
        if (has && length(cand) > 96) {
          print line
          line = ind w[i]
        } else {
          line = cand
        }
        has = 1
      }
      print line
    }'
}

para() { printf '%s\n' "$1" | wrap ''; }
quote() { printf '%s\n' "$1" | wrap '> ' '> '; }
item() { printf '%s\n' "$1" | wrap '- '; }
box() { printf '%s\n' "$1" | wrap '- [ ] '; }
step() { printf '%s\n' "$2" | wrap "${1}. "; }

# One line per bullet, unwrapped: the body builder pipes these through item.
area_paths() {
  case "$1" in
    "area: model") cat <<'TXT'
`internal/model/`: the types every other package imports, and the one package that imports nothing from this module. `exit.go` is where the FR-10 mapping lives.
`internal/model/model_test.go`: where a new field or severity ordering gets its table test.
TXT
      ;;
    "area: strkey") cat <<'TXT'
`internal/strkey/`: `address.go`, `checksum.go` and `seed.go`, the three files that decide which keys this tool will look at at all.
`internal/strkey/fuzz_test.go`: the `FuzzClassify` target, run as a smoke pass rather than a CI job.
TXT
      ;;
    "area: threshold") cat <<'TXT'
`internal/threshold/threshold.go`: `Reachable` and `MinSigners`, pure functions whose semantics task 0.7 verified.
`internal/threshold/threshold_test.go`: the table tests and the brute-force cross-check that goes with them.
`ARCHITECTURE.md`: the section the essentials item on threshold math cites.
TXT
      ;;
    "area: policy") cat <<'TXT'
`internal/policy/config.go` and `decode.go`: the strict decode, where an unknown key has to become a refusal rather than a default.
`internal/policy/roles.go` and `accept.go`: Appendix A role profiles and the accepted-risk entries.
`gauge.example.toml`: the shipped example, updated with every config field.
TXT
      ;;
    "area: sanitize") cat <<'TXT'
`internal/sanitize/text.go`, `domain.go` and `markdown.go`: control, bidi and zero-width handling, and the markup escape.
`internal/sanitize/fuzz_test.go`: the `FuzzText` and `FuzzFenced` targets.
TXT
      ;;
    "area: source") cat <<'TXT'
`internal/source/`: `http.go` for the caps and retries, `horizon.go` for the decode, `errors.go` for the refusal types. A protected path.
`internal/source/horizon_test.go`: the fake-server tests, one per failure mode.
`testdata/horizon/`: captured and built fixtures, with `README.md` stating which is which and what stays uncaptured.
TXT
      ;;
    "area: sep1") cat <<'TXT'
`internal/sep1/guard.go`: the SSRF guard every `stellar.toml` fetch passes through. A protected path.
`internal/sep1/fetch.go` and `parse.go`: the fetch and the field reading 0.8 pinned.
`internal/sep1/guard_test.go` and `fetch_test.go`: private-IP, redirect, oversize and malformed cases.
TXT
      ;;
    "area: rules") cat <<'TXT'
`internal/rules/`: one file per family, `account.go`, `signers.go`, `thresholds.go`, `flags.go`, `identity.go` and `facts.go`, registered in `rules.go`.
`testdata/horizon/` and `testdata/golden/`: the positive and negative fixture pair, and the goldens a new severity moves.
`PRD.md` 6.1 and `ARCHITECTURE_ESSENTIALS.md` 8: the rule list and the short form that has to agree with it.
TXT
      ;;
    "area: report") cat <<'TXT'
`internal/report/`: `text.go`, `json.go` and `exit.go`, with `markdown.go` still a one-line scaffold stub.
`testdata/golden/`: the expected output of both renderers, regenerated only with `-update`.
`internal/report/report_test.go`: determinism and the sanitizer's place in the render path.
TXT
      ;;
    "area: run") cat <<'TXT'
`internal/run/`: `run.go`, `check.go` and `audit.go`, with `compare.go`, `diff.go` and `snapshot.go` still one-line stubs.
`cmd/stellar-gauge/`: the subcommand beside it, since orchestration and flag handling move together.
`internal/run/run_test.go` and `scenarios_test.go`: the fail-closed and Appendix B expectations.
TXT
      ;;
    "area: cli") cat <<'TXT'
`cmd/stellar-gauge/`: `main.go`, `check.go`, `audit.go`, `version.go`, with `diff.go` and `snapshot.go` still one-line stubs.
`cmd/stellar-gauge/cli_test.go` and `g3_test.go`: flag behavior and the gate G3 fail-closed paths.
TXT
      ;;
    "area: ci") cat <<'TXT'
`.github/workflows/`: `ci.yml`, `roadmap-check.yml`, `scorecard.yml`, `pages.yml`, and any new job this row adds.
`scripts/`: the gate scripts themselves, which is where a backlog or roadmap check belongs.
`SECURITY.md` and `ARCHITECTURE.md` 11: the claims a CI change makes about what runs.
TXT
      ;;
    *) cat <<'TXT'
`README.md` and the seven briefing documents: `PRD.md`, `ARCHITECTURE.md`, `ARCHITECTURE_ESSENTIALS.md`, `project_structure.md`, `AGENTS.md`, `CLAUDE.md`, plus `CONTRIBUTING.md` and `SECURITY.md`.
`site/`: the GitHub Pages set deployed by `.github/workflows/pages.yml`.
`docs/`: the long-form documents that do not belong in the root set, including `WAVE_SUBMISSION.md`, `RELEASE.md` and `RECORDING.md`.
TXT
      ;;
  esac
}

area_rules() {
  case "$1" in
    "area: strkey") cat <<'TXT'
Accept only `G...` addresses; reject `M...` and `C...` with a message that names which one arrived. Enforced by the table tests in `internal/strkey/address_test.go`.
Never accept, store, log or echo a secret seed. Reject it with exit 2. `seed.go` is the refusal and `model/exit.go` is the number.
Tests never use a real seed, even a testnet one: seed-shaped strings are built from fragments at test time. `AGENTS.md` section 4.
TXT
      ;;
    "area: threshold") cat <<'TXT'
Do not change the threshold semantics beyond what task 0.7 verified. The brute-force cross-check in `internal/threshold/threshold_test.go` is the guard, and it is the reason the package is protected.
Rules stay pure functions: `threshold` takes a snapshot and returns a decision, with no I/O, no globals, no time and no randomness.
TXT
      ;;
    "area: policy") cat <<'TXT'
The network is always explicit and an unknown config key is a refusal, not a default: exit 2 with a message naming the key. `config_test.go` and `decode.go` hold that line.
Only `github.com/BurntSushi/toml` is allowed, and only inside `policy` and `sep1`. A new module needs the owner's approval before the work.
Only the owner adds `[[accept]]` entries, always with a reason. A contributor proposing one writes it in the pull request rather than committing it.
TXT
      ;;
    "area: sanitize" | "area: report") cat <<'TXT'
Every chain-derived string is sanitized before it reaches any output, and the report never calls an account secure or safe. It describes posture, and carries the limits footer.
Output changes keep the goldens in sync, and goldens regenerate only with `-update` after reading the diff. Fingerprints pin wording, so a prose edit to a rule is a golden diff by design.
TXT
      ;;
    "area: source") cat <<'TXT'
Every HTTP call has a timeout, a size cap, bounded retries and bounded concurrency. `internal/source/http.go` implements them; a change there has to keep them observable in tests.
Verify the endpoint's passphrase. The network is never inferred, and a mismatch is a finding rather than a fallback.
Never log a response body. Tests stay off the real network unless tagged `livenet`.
TXT
      ;;
    "area: sep1") cat <<'TXT'
`stellar.toml` is fetched only through the SSRF-guarded client, and a hostile hostname string is refused before a socket opens. `guard_test.go` is the proof.
Every HTTP call keeps its timeout, size cap and bounded retries. A new fetch is not a licence to add an unbounded one.
Never log a response body: a `stellar.toml` is untrusted input, and the report cites the finding rather than the file.
TXT
      ;;
    "area: rules") cat <<'TXT'
Rules are pure functions: no I/O, no globals, no time, no randomness. `project_structure.md` section 3 states the import direction that keeps `rules` away from `source` and `net/http`.
Every rule has a positive and a negative fixture, and every rule change has a golden diff beside it in the same pull request.
No new dependency and no network call inside a rule. A rule reads what the caller already fetched.
TXT
      ;;
    "area: run" | "area: cli") cat <<'TXT'
Never return exit 0 after an unreachable endpoint, an oversized or malformed response, an unknown config key or a panic. FR-10 is the mapping and `cmd/stellar-gauge/g3_test.go` is the test that keeps it honest.
No writes except an explicit `--out`, and `--out` is a reserved name rather than a shipped flag today. `check` and `audit` write only to stdout and stderr.
Accept only `G...` addresses at the flag boundary; reject `M...`, `C...` and any `S...` seed with exit 2.
TXT
      ;;
    "area: ci") cat <<'TXT'
Actions and tools are pinned by commit SHA, and no pin moves without reading the diff between the two SHAs. `.github/workflows/ci.yml` shows the form.
Every job keeps `permissions: contents: read` or narrower, and a new job states why it needs what it asks for.
The roadmap gate has no exemptions, including dependency bumps. That is D-08, and it is why this row cannot be satisfied by skipping a check.
TXT
      ;;
    *) cat <<'TXT'
A claim names the run that produced it or says it is local, including in a document. An `[Unverified]` statement stays `[Unverified]` until the Phase 0 row that verifies it runs.
No em dashes in prose, comments or commit messages, and output wording describes posture rather than safety.
If two documents disagree, stop and ask rather than picking a side in a diff. `AGENTS.md` section 10.
TXT
      ;;
  esac
  printf 'Never weaken, skip or delete a check or a test to make something pass. If a test is wrong, fix it and say why in the pull request. `AGENTS.md` section 4.\n'
}

area_read() {
  case "$1" in
    "area: rules") printf 'PRD.md 6.1 and Appendix A and B: the rule list, the role severities and the numbered scenarios a rule has to answer.' ;;
    "area: source" | "area: sep1") printf 'ARCHITECTURE.md: the section the essentials item on HTTP hygiene cites, then the fake-server tests that already cover each failure mode.' ;;
    "area: threshold" | "area: model") printf 'ARCHITECTURE.md on the models, and `PRD.md` FR-04 or FR-10 as they apply to this package.' ;;
    "area: policy") printf 'gauge.example.toml, then `ARCHITECTURE.md` 7 on the config surface and its version field.' ;;
    "area: report" | "area: sanitize") printf 'One golden pair under `testdata/golden/`, and the limits footer `internal/report/text.go` writes.' ;;
    "area: run" | "area: cli") printf 'cmd/stellar-gauge/main.go for the flag set, then `README.md` for what is documented as shipped.' ;;
    "area: ci") printf '`.github/workflows/ci.yml` and `roadmap-check.yml`, then `scripts/check_roadmap_touched.sh` to see what the gate already enforces.' ;;
    *) printf 'docs/WAVE_SUBMISSION.md and the `ROADMAP.md` decision rows it cites: D-07, D-10, D-13, D-14.' ;;
    esac
}

handoff_checks() {
  case "$1" in
    "area: rules" | "area: model" | "area: threshold" | "area: strkey" | "area: sanitize" | "area: policy") cat <<'TXT'
the failing test or fixture arrives before the change, not after it
the golden or table diff is explained line by line
nothing in a protected path got looser
TXT
      ;;
    "area: source" | "area: sep1" | "area: run" | "area: cli" | "area: report") cat <<'TXT'
a fake-server or exit-code test covers the new failure mode
no response body reaches a log and no seed reaches an input path
the limits footer and the wording still say posture
TXT
      ;;
    "area: ci") cat <<'TXT'
every action and tool is pinned by SHA, with the diff read
permissions stay `contents: read` or narrower
no existing check got dropped, reordered or made optional
TXT
      ;;
    *) cat <<'TXT'
every factual claim names its run or says it is local
no em dash anywhere in the diff
`ROADMAP.md` moved in the same pull request, with the table regenerated
TXT
      ;;
  esac
}

protected_note() {
  case "$1" in
    "area: model" | "area: strkey" | "area: threshold" | "area: policy" | "area: sanitize" | "area: source" | "area: sep1" | "area: rules")
      item 'This area is a protected path (`project_structure.md` section 2): the owner reviews it, and nothing in it may be loosened to make the check pass.'
      ;;
    "area: ci")
      item '`.github/workflows/` and `scripts/` are protected paths: the owner reviews them, and a gate may only gain a check here.'
      ;;
    *) item 'Nothing here is a protected path, but `ROADMAP.md` and every briefing document it cites still need the owner'"'"'s read before a rule or decision changes.' ;;
  esac
}

# "4.1 to 4.6" style ranges expand; anything the parser cannot name is treated
# as not done, which keeps an unlisted dependency from qualifying as clear.
dep_ticked() {
  local dl
  dl="$(roadmap_line "$1")"
  [ -n "${dl}" ] && [ "$(printf '%s' "${dl}" | cut -c4)" = "x" ]
}

deps_done() {
  local raw tok rng a b i ok=1
  [ -z "${1}" ] && return 0
  raw="$(printf '%s' "${1}" | tr ',' '\n' | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
  while IFS= read -r tok; do
    [ -n "${tok}" ] || continue
    rng="$(printf '%s' "${tok}" | sed -nE 's/^([0-9]+\.[0-9]+) to ([0-9]+\.[0-9]+)$/\1:\2/p')"
    if [ -n "${rng}" ]; then
      a="${rng%%:*}" b="${rng##*:}"
      [ "${a%%.*}" = "${b%%.*}" ] || { ok=0; break; }
      i="${a##*.}"
      while [ "${i}" -le "${b##*.}" ]; do
        dep_ticked "${a%%.*}.${i}" || { ok=0; break; }
        i=$((i + 1))
      done
      [ "${ok}" -eq 1 ] || break
      continue
    fi
    case "${tok}" in
      [0-9]*.[0-9]*) dep_ticked "${tok}" || { ok=0; break; } ;;
      *) ok=0; break ;;
    esac
  done < <(printf '%s\n' "${raw}")
  [ "${ok}" -eq 1 ]
}

# Contributor routing labels, applied by this script so a rerun cannot wipe them.
# Only tasks whose dependencies are already ticked and that need nothing beyond
# public documentation qualify: the Phase 0 research tasks a newcomer can finish
# without touching a network, a decision or a protected path.
good_first="0.5 0.8 0.9 0.12 0.13"
help_wanted="${good_first} 0.6 2.1"

routing_labels() {
  out=""
  case " ${help_wanted} " in *" $1 "*) out="${out},help wanted" ;; esac
  case " ${good_first} " in *" $1 "*) out="${out},good first issue" ;; esac
  printf '%s' "${out#,}"
}

load_prose() {
  while IFS=$'\t' read -r pid pkind pbrief pwhy pscope; do
    [ -n "${pid}" ] || continue
    prose_kind["${pid}"]="${pkind}"
    prose_brief["${pid}"]="${pbrief}"
    prose_why["${pid}"]="${pwhy}"
    prose_scope["${pid}"]="${pscope}"
  done < <(issue_prose)
}

declare -A prose_kind=() prose_brief=() prose_why=() prose_scope=()
load_prose

# Fills the globals the body is built from for the row currently in ${line}.
parse_row() {
  text="$(printf '%s' "${line}" | sed -E 's/^- \[[ x]\] \*\*[^*]+\*\* //' | strip_backticks)"
  size="$(printf '%s' "${text}" | grep -oE '(^| )(XL|L|M|S)\. ' | head -1 | tr -d '. ' || true)"
  summary="$(printf '%s' "${text}" | sed -E 's/ (XL|L|M|S)\. (Deps:|Done when:).*//; s/ Done when:.*//; s/[[:space:]]*$//')"
  deps="$(printf '%s' "${text}" | sed -nE 's/.*Deps: (.*)\. Done when:.*/\1/p')"
  [ -n "${deps}" ] || deps="$(printf '%s' "${text}" | sed -nE 's/.*Deps: (.*)$/\1/p' | sed -E 's/\.$//')"
  done_when="$(printf '%s' "${text}" | sed -nE 's/.*Done when: //p')"
  # A row's dated commentary is the prose that follows its acceptance clause,
  # either in a trailing parenthesis or as a new sentence. It stays in the
  # roadmap and gets named in the body; it is not an acceptance box.
  row_commentary=0
  if printf '%s' "${done_when}" | grep -q ' ('; then
    row_commentary=1
    done_when="$(printf '%s' "${done_when}" | sed -E 's/ \(.*$//')"
  elif printf '%s' "${done_when}" | grep -qE '[a-z]\. [A-Z]'; then
    row_commentary=1
    done_when="$(printf '%s' "${done_when}" | sed -E 's/([a-z])\. [A-Z].*/\1/')"
  fi
  done_when="$(printf '%s' "${done_when}" | sed -E 's/\.$//; s/[[:space:]]+$//')"
  complexity="$(complexity_for "${size}")"
  tier="$(tier_for "${size}")"
  case "${id}" in
    G*) row_section="section 3" ;;
    *) row_section="section 5" ;;
  esac
}

# The PI-1 body for the task in ${id}, from the roadmap row in ${line} and the
# prose tables above. Everything it prints is either in one of those two places
# or derived from the area label.
build_body() {
  local kind brief wave_tier=""
  kind="${prose_kind[${id}]}"
  brief="${prose_brief[${id}]}"

  if [ "${kind}" = "task" ] && [ -n "${tier}" ] && deps_done "${deps}"; then
    wave_tier="${tier}"
  fi

  printf '<!-- posture-issue: %s/%s | contract: PI-1 | kind: %s' "${repo_name}" "${id}" "${kind}"
  [ -n "${wave_tier}" ] && printf ' | wave: %s' "${wave_tier}"
  printf ' -->\n\n'
  quote "${brief}"
  printf '\n'

  printf '## 1 · Why this exists\n\n'
  para "${prose_why[${id}]}"
  printf '\n'

  printf '## 2 · Done when\n\n'
  case "${kind}" in
    gate)
      box 'The observation that closes this gate is written in a comment here: the run id or the setting read, its date in UTC, and what it printed.'
      box 'The `ROADMAP.md` row is ticked in the same pull request, or the reason it cannot be ticked yet is recorded in the changelog.'
      printf '\n'
      para 'A gate closes on a reading, not on a diff. Nobody should tick this row by shipping code that claims the gate holds.'
      ;;
    owner)
      box 'The owner performs the act and records it in a comment here with the date in UTC.'
      box "$(printf '`ROADMAP.md` task %s is ticked in the same commit that records it, in %s.' "${id}" "${row_section}")"
      printf '\n'
      para 'This row is an act, not a task: only the account owner can take it, so a contributor should not start work on it. Comment if the row is blocking something you can do.'
      ;;
    *)
      if [ -n "${done_when}" ]; then
        printf '%s\n' "${done_when}" | tr ';' '\n' | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' \
          | grep -v '^$' | while IFS= read -r clause; do box "${clause}"; done
      else
        box "$(printf '%s' "${summary}" | sed -E 's/\.$//')"
      fi
      box "$(printf '`ROADMAP.md` task %s is ticked in this pull request, with a changelog row stamped from `date -u` and the progress table regenerated.' "${id}")"
      if [ "${row_commentary}" -eq 1 ]; then
        printf '\n'
        para 'The row has more after its clause: a gate cross-reference or dated commentary. Read the whole row in `ROADMAP.md` before treating the boxes above as the whole condition.'
      fi
      ;;
  esac
  printf '\n'
  para "Source of truth: \`ROADMAP.md\` ${row_section}, task ${id}. If this issue and that row disagree, the row is right and this issue is stale."
  printf '\n'

  printf '## 3 · Where the work lands\n\n'
  area_paths "${tarea}" | while IFS= read -r l; do item "${l}"; done
  printf '\n'

  printf '## 4 · Read before writing\n\n'
  step 1 'ROADMAP.md: this task'"'"'s row in '"${row_section}"', section 4 on current focus, and the decision-log entries named above.'
  step 2 'AGENTS.md sections 4 and 5 and ARCHITECTURE_ESSENTIALS.md section 2: the hard rules and the commands that prove them. Nothing here overrides them.'
  step 3 'project_structure.md sections 2, 4 and 5: protected paths, where a change belongs, and how rules, findings and fixtures are named.'
  step 4 "$(area_read "${tarea}")"
  step 5 'Setup: Go 1.27.3 from `go.mod`, then `go install` the pinned `staticcheck` v0.8.2 and `govulncheck` v1.9.0.'
  printf '\n'

  # A gate binds a reading rather than a diff, and the contract marks that in the heading.
  if [ "${kind}" = "gate" ]; then
    printf '## 5 · Rules that bind this gate\n\n'
  else
    printf '## 5 · Rules that bind this issue\n\n'
  fi
  area_rules "${tarea}" | while IFS= read -r l; do item "${l}"; done
  printf '\n'

  printf '## 6 · How to prove it\n\n'
  cat <<'TXT'
```
$ gofmt -l .                       # must print nothing
$ go vet ./...
$ staticcheck ./...                # v0.8.2
$ go test ./... -race -count=1
$ govulncheck ./...                # v1.9.0
$ scripts/check_roadmap_touched.sh origin/main
$ scripts/roadmap_progress.sh --check ROADMAP.md
$ scripts/test_roadmap_gate.sh
$ scripts/generate_issues.sh --check
$ scripts/test_issue_shape.sh
```

TXT
  case "${kind}" in
    gate)
      item 'Local half: the read you can do at a desk, and the query that shows the current state. Paste what it printed with the UTC date, and name what you could not reach.'
      item 'Hosted half: the workflow runs on this repository'"'"'s default branch and the organization settings behind them. Only the owner can pull those records; if that is the missing half, say which job you need and what it would show.'
      ;;
    owner)
      item 'Local half: nothing. This row is not desk work; it is an act in GitHub'"'"'s account or organization settings, or a statement the owner makes under his own name.'
      item 'Hosted half: the owner records the observation here and ticks the row. A contributor cannot close it and should not open a pull request against it.'
      ;;
    *)
      item 'Local half: everything in the block above, on your machine, before the pull request opens. Paste the real output in the testing section rather than describing it.'
      item 'Hosted half: `ci / go`, `ci / vuln` and `roadmap-check / gate` for your pull request. A fork'"'"'s green run is evidence about a fork: merging and anything that reads protected settings needs write access here.'
      ;;
  esac
  printf '\n'

  printf '## 7 · Out of scope\n\n'
  para "${prose_scope[${id}]}"
  item 'A new module, a bump of a pinned action, or a change to a refusal in a validator needs the owner'"'"'s agreement before the work, not in the pull request after it.'
  printf '\n'

  printf '## 8 · Handoff\n\n'
  item "Branch \`$(verb_for "${ttype}")/${id}-<slug>\` from \`main\`: the slug is this title's."
  item 'Fill `.github/pull_request_template.md` by quoting section 2 and section 6, not rewriting them.'
  item 'A reviewer checks three things first:'
  handoff_checks "${tarea}" | while IFS= read -r c; do printf '  - %s\n' "${c}"; done
  protected_note "${tarea}"
  if [ -n "${wave_tier}" ]; then
    item 'Drips Wave: the maintainer enters the tier for this issue in the dashboard, never through a GitHub label.'
  fi
  printf '\n'

  printf '## 9 · When to stop and ask\n\n'
  case "${kind}" in
    gate) item 'The observation this gate wants is out of reach from where you are: a run log, a setting or an artifact only the organization can see.' ;;
    owner) item 'You are about to do this work at all. Comment instead, and say what the row is blocking for you.' ;;
    *)
      if [ -n "${deps}" ] && ! deps_done "${deps}"; then
        item "A \`Deps:\` clause is not ticked yet: ${deps}."
      fi
      item 'The task needs a new module, a new network call, or a loosening anywhere in a protected path.'
      ;;
  esac
  item 'Two documents disagree, or a claim here depends on an `[Unverified]` statement.'
  item 'A hard rule in `AGENTS.md` section 4 appears to conflict with the row. D-07 and D-15 are the two decisions still open in the log.'
  para 'Then ask here: name the rule or the row that blocks you, and stop. Do not open a pull request that works around a hard rule.'
}

created=0
updated=0
planned=0
skipped=0
rows_seen=""

# id to issue number for whatever the backlog already created. The contract's
# marker is what a new body carries; the pre-PI-1 form stays matched so a
# migration run finds the issues it is rewriting instead of opening doubles.
declare -A issue_of=()
if [ "${apply}" -eq 1 ] || [ "${update}" -eq 1 ]; then
  while read -r num marker; do
    [ -n "${marker}" ] && issue_of["${marker}"]="${num}"
  done < <(gh issue list --repo "${repo}" --state all --limit 500 --json number,body \
    --jq '.[] | "\(.number) \(.body | split("\n")[0])"' 2>/dev/null | sed -nE \
      -e 's/^([0-9]+) .*posture-issue: [^ ]*\/([^ ]+) .*/\1 \2/p' \
      -e 's/^([0-9]+) .*gauge-task: ([^ ]+).*/\1 \2/p')
fi

if [ -n "${body_for}" ]; then
  while IFS=$'\t' read -r id ttype tarea title; do
    [ "${id}" = "${body_for}" ] || continue
    line="$(roadmap_line "${id}")"
    [ -n "${line}" ] || { echo "generate_issues: no roadmap task ${id}" >&2; exit 1; }
    [ -n "${prose_kind[${id}]:-}" ] || {
      echo "generate_issues: no PI-1 prose row for ${id}; the contract forbids a generated section 1" >&2
      exit 2
    }
    parse_row
    build_body
    exit 0
  done < <(backlog)
  echo "generate_issues: no backlog row for ${body_for}" >&2
  exit 1
fi

while IFS=$'\t' read -r id ttype tarea title; do
  [ -n "${id}" ] || continue
  line="$(roadmap_line "${id}")"

  if [ "${check}" -eq 1 ]; then
    if [ -z "${line}" ]; then
      echo "generate_issues: row ${id} has no task in ${roadmap}" >&2
      exit 1
    fi
    rows_seen="${rows_seen}${id} "
    continue
  fi

  if [ -z "${line}" ]; then
    echo "generate_issues: no roadmap task ${id}" >&2
    exit 1
  fi

  state="$(printf '%s' "${line}" | cut -c4)"
  if [ "${state}" = "x" ] && [ "${include_done}" -eq 0 ]; then
    skipped=$((skipped + 1))
    continue
  fi

  case "${id}" in
    G*) row_phase="gates" ;;
    *) row_phase="${id%%.*}" ;;
  esac
  if [ -n "${phase}" ] && [ "${row_phase}" != "${phase}" ]; then
    continue
  fi

  parse_row

  if [ -z "${prose_kind[${id}]:-}" ]; then
    echo "generate_issues: ${id} has no PI-1 prose row in issue_prose; add the kind, brief, why and scope before opening it" >&2
    exit 2
  fi

  existing="${issue_of[${id}]:-}"
  if [ -n "${existing}" ] && [ "${update}" -eq 0 ]; then
    skipped=$((skipped + 1))
    continue
  fi

  labels="${ttype},${tarea}"
  [ -n "${complexity}" ] && labels="${labels},${complexity}"
  routing="$(routing_labels "${id}")"
  [ -n "${routing}" ] && labels="${labels},${routing}"

  slug="$(printf '%s' "${title}" | sed -E 's/^[a-z]+(\([a-z0-9]+\))?: //' | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9]+/-/g; s/^-//; s/-$//' | cut -d- -f1-5)"

  body="$(mktemp)"
  build_body >"${body}"

  if [ "${apply}" -eq 1 ] && [ -n "${existing}" ]; then
    payload="$(mktemp)"
    jq -n --arg title "${title}" --arg body "$(cat "${body}")" \
      --argjson labels "$(printf '%s' "${labels}" | tr ',' '\n' | jq -R . | jq -s .)" \
      '{title: $title, body: $body, labels: $labels}' >"${payload}"
    gh api --method PATCH "repos/${repo}/issues/${existing}" --input "${payload}" \
      --jq '"#" + (.number | tostring)' </dev/null >/dev/null
    rm -f "${payload}"
    echo "updated ${id}: issue #${existing}"
    updated=$((updated + 1))
  elif [ "${apply}" -eq 1 ]; then
    url="$(gh issue create --repo "${repo}" --title "${title}" --label "${labels}" --body-file "${body}" </dev/null)"
    echo "created ${id}: ${title} -> ${url}"
    created=$((created + 1))
  elif [ -n "${existing}" ]; then
    echo "would update ${id}: issue #${existing} [${labels}]"
  else
    echo "would create ${id}: ${title} [${labels}]"
  fi
  rm -f "${body}"
  planned=$((planned + 1))
done < <(backlog)

if [ "${check}" -eq 1 ]; then
  missing="$(awk '/^- \[[ ]\] \*\*(G[0-9]+|[0-6]\.[0-9]+)\*\*/ {
      id = substr($0, 9); sub(/\*\*.*/, "", id); print id
    }' "${roadmap}" | while read -r id; do
      printf '%s\n' "${rows_seen}" | grep -qw -- "${id}" || printf '%s\n' "${id}"
    done | tr '\n' ' ')"
  if [ -n "${missing}" ]; then
    echo "generate_issues: open tasks with no backlog row: ${missing}" >&2
    exit 1
  fi
  # An open task also needs curated prose, or the backlog cannot describe why it
  # exists. This is the check that stops a new row from minting an empty issue.
  noprose="$(awk '/^- \[[ ]\] \*\*(G[0-9]+|[0-6]\.[0-9]+)\*\*/ {
      id = substr($0, 9); sub(/\*\*.*/, "", id); print id
    }' "${roadmap}" | while read -r id; do
      [ -n "${prose_kind[${id}]:-}" ] || printf '%s\n' "${id}"
    done | tr '\n' ' ')"
  if [ -n "${noprose}" ]; then
    echo "generate_issues: open tasks with no PI-1 prose row: ${noprose}" >&2
    exit 1
  fi
  echo "generate_issues: backlog covers every open task in ${roadmap}"
  exit 0
fi

if [ "${apply}" -eq 1 ]; then
  echo "generate_issues: ${created} created, ${updated} updated, ${skipped} already present and unchanged"
else
  echo "generate_issues: ${planned} issue(s) planned, ${skipped} already present or ticked (dry run)"
fi
