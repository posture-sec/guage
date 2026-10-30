#!/usr/bin/env bash
# Builds the planned GitHub issue backlog from the open tasks in ROADMAP.md.
#
# One curated row per task ID supplies the title, type and area labels. The body
# is read from ROADMAP.md at run time, so acceptance criteria cannot drift away
# from the plan. A hidden marker line in each body keeps the backlog idempotent:
# rerunning the script only creates tasks that have no issue yet.
#
# Usage: scripts/generate_issues.sh [options] [command]
#   --phase N       limit to one phase (0 to 6, or "gates")
#   --roadmap FILE  roadmap to read (default ROADMAP.md)
#   --repo OWNER/NAME  target repository (default posture-sec/guage)
#   --apply         create the missing issues with gh
#   --check         fail if an open task has no row, or a row has no open task
#   --include-done  also emit rows for tasks already ticked
set -euo pipefail

repo="${GUAGE_REPO:-posture-sec/guage}"
roadmap="ROADMAP.md"
phase=""
apply=0
check=0
include_done=0

while [ $# -gt 0 ]; do
  case "$1" in
    --phase) phase="${2:?--phase needs a value}"; shift ;;
    --roadmap) roadmap="${2:?--roadmap needs a value}"; shift ;;
    --repo) repo="${2:?--repo needs a value}"; shift ;;
    --apply) apply=1 ;;
    --check) check=1 ;;
    --include-done) include_done=1 ;;
    *) echo "generate_issues: unknown argument ${1}" >&2; exit 2 ;;
  esac
  shift
done

if [ ! -f "${roadmap}" ]; then
  echo "generate_issues: ${roadmap} not found" >&2
  exit 1
fi

# id, type, area, title. Titles stay under 72 characters and use the
# Conventional Commits prefix that matches the type label.
backlog() {
  printf '%s\n' \
    '0.2	type: research	area: docs	docs(decisions): record name, org and license choices' \
    '0.3	type: research	area: docs	docs(decisions): choose code sequencing against Airlock v0.1.0' \
    '0.4	type: research	area: docs	docs(demand): validate demand with three real operators' \
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

# Roadmap sizes are unmeasured guesses, so the label buckets stay coarse: S and
# M share medium, L and XL share high. complexity: trivial is reserved for fixes
# a reviewer can check in one screen, which no roadmap task claims to be.
# The body always prints the roadmap size next to the label.
complexity_for() {
  case "$1" in
    L | XL) printf 'complexity: high' ;;
    S | M) printf 'complexity: medium' ;;
    *) printf '' ;;
  esac
}

created=0
planned=0
skipped=0
rows_seen=""

known_markers=""
if [ "${apply}" -eq 1 ]; then
  known_markers="$(gh issue list --repo "${repo}" --state all --limit 500 --json body \
    --jq '.[].body' 2>/dev/null | sed -nE 's/.*gauge-task: ([^ ]+).*/\1/p' | sort -u || true)"
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

  text="$(printf '%s' "${line}" | sed -E 's/^- \[[ x]\] \*\*[^*]+\*\* //' | strip_backticks)"
  size="$(printf '%s' "${text}" | grep -oE '(^| )(XL|L|M|S)\. ' | head -1 | tr -d '. ' || true)"
  summary="$(printf '%s' "${text}" | sed -E 's/ (XL|L|M|S)\. (Deps:|Done when:).*//; s/ Done when:.*//; s/[[:space:]]*$//')"
  deps="$(printf '%s' "${text}" | sed -nE 's/.*Deps: (.*)\. Done when:.*/\1/p')"
  [ -n "${deps}" ] || deps="$(printf '%s' "${text}" | sed -nE 's/.*Deps: (.*)$/\1/p' | sed -E 's/\.$//')"
  done_when="$(printf '%s' "${text}" | sed -nE 's/.*Done when: //p' | sed -E 's/\.$//')"
  complexity="$(complexity_for "${size}")"

  if printf '%s\n' "${known_markers}" | grep -qx -- "${id}"; then
    skipped=$((skipped + 1))
    continue
  fi

  labels="${ttype},${tarea}"
  [ -n "${complexity}" ] && labels="${labels},${complexity}"

  slug="$(printf '%s' "${title}" | sed -E 's/^[a-z]+(\([a-z0-9]+\))?: //' | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9]+/-/g; s/^-//; s/-$//' | cut -d- -f1-5)"
  verb="$(verb_for "${ttype}")"

  body="$(mktemp)"
  {
    printf '<!-- gauge-task: %s -->\n\n' "${id}"
    printf '## Summary\n\n%s\n\n' "${summary}"
    printf '## Acceptance criteria\n\n'
    if [ -n "${done_when}" ]; then
      printf '%s\n' "${done_when}" | tr ';' '\n' | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' \
        | grep -v '^$' | sed -E 's/^/- [ ] /'
    else
      printf '%s\n' "${summary}" | sed -E 's/\.$//' | sed -E 's/^/- [ ] /'
    fi
    printf -- '- [ ] `ROADMAP.md` task %s is ticked in the same pull request\n\n' "${id}"
    printf '## Dependencies\n\n'
    if [ -n "${deps}" ]; then
      printf '%s\n\n' "${deps}"
    else
      printf 'None recorded.\n\n'
    fi
    printf '## Roadmap line\n\n'
    printf '> %s\n\n' "${text}"
    printf '## Working notes\n\n'
    printf -- '- Source of truth: `ROADMAP.md`, %s task `%s` (size: %s).\n' \
      "$( [ "${row_phase}" = "gates" ] && printf 'section 3,' || printf 'section 5, Phase %s,' "${row_phase}")" \
      "${id}" "${size:-not stated}"
    printf -- '- Suggested branch: `%s/%s-%s`.\n' "${verb}" "${id}" "${slug}"
    printf -- '- Read `AGENTS.md` before starting: section 4 holds the hard rules, section 5 the checks to run.\n'
    printf -- '- Write the failing test or fixture first, then the smallest change that passes it.\n'
    printf -- '- If a hard rule or an open decision blocks this task, stop and ask the owner.\n\n'
    printf '## Tech stack\n\n'
    printf -- '- Go %s, module `%s`\n' "$(sed -nE 's/^go ([0-9.]+)$/\1/p' go.mod)" "$(sed -nE 's/^module (\S+)$/\1/p' go.mod)"
    printf -- '- Standard library plus `github.com/BurntSushi/toml` only; a new module needs owner approval\n'
    printf -- '- Checks: `gofmt`, `go vet`, `staticcheck` v0.8.2, `go test -race`, `govulncheck` v1.9.0\n'
    printf -- '- GitHub Actions pinned by commit SHA for CI and release tasks\n'
  } >"${body}"

  if [ "${apply}" -eq 1 ]; then
    url="$(gh issue create --repo "${repo}" --title "${title}" --label "${labels}" --body-file "${body}" </dev/null)"
    echo "created ${id}: ${title} -> ${url}"
    created=$((created + 1))
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
  echo "generate_issues: backlog covers every open task in ${roadmap}"
  exit 0
fi

if [ "${apply}" -eq 1 ]; then
  echo "generate_issues: ${created} issue(s) created, ${skipped} already present or ticked"
else
  echo "generate_issues: ${planned} issue(s) planned, ${skipped} ticked or already present (dry run)"
fi
