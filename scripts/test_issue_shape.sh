#!/usr/bin/env bash
# Self-test for the PI-1 issue generator. Reads the open tasks from ROADMAP.md,
# builds each body with scripts/generate_issues.sh --body-for (no network, no
# gh) and checks the shape the contract in posture-sec/.github promises.
#
# The length window is the contract's own: it aims at 2500 to 5000 characters
# and says that above 6000 an issue has become a tutorial. This gate enforces
# the floor and the ceiling, not the aim, so a body over the aim fails here only
# when it has outgrown the contract rather than when it ran long.
set -uo pipefail

roadmap="ROADMAP.md"
gen="scripts/generate_issues.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT
failures=0

note() { printf '%s\n' "$1" >&2; }

fail() {
  failures=$((failures + 1))
  printf 'FAIL %s\n' "$1" >&2
}

# The nine headings, in the contract's order.
headings=(
  '## 1 · Why this exists'
  '## 2 · Done when'
  '## 3 · Where the work lands'
  '## 4 · Read before writing'
  '## 5 · Rules that bind this'
  '## 6 · How to prove it'
  '## 7 · Out of scope'
  '## 8 · Handoff'
  '## 9 · When to stop and ask'
)

check_body() {
  local id="$1" file="$2" kind line
  local chars

  line="$(head -1 "${file}")"
  if ! printf '%s' "${line}" | grep -qE '^<!-- posture-issue: guage/'"${id}"' \| contract: PI-1 \| kind: (task|gate|owner|report)( \| wave: (Trivial|Medium|High))? -->$'; then
    fail "${id}: marker on line 1 is not the contract's form: ${line}"
  fi
  kind="$(printf '%s' "${line}" | sed -E 's/.*kind: ([a-z]+).*/\1/')"

  local want=0
  for h in "${headings[@]}"; do
    want=$((want + 1))
    if ! grep -qF "${h}" "${file}"; then
      fail "${id}: missing section ${want} (${h})"
    fi
  done
  # Order: the section numbers must appear ascending, each exactly once.
  if [ "$(grep -cE '^## [0-9]+ · ' "${file}")" -ne 9 ]; then
    fail "${id}: $(grep -cE '^## [0-9]+ · ' "${file}") numbered headings, expected nine"
  fi
  if [ "$(grep -oE '^## [0-9]+' "${file}" | tr -d '# ' | tr '\n' ' ')" != "1 2 3 4 5 6 7 8 9 " ]; then
    fail "${id}: sections are not in the contract's order"
  fi

  # Section 5 is "this issue" for tasks and "this gate" for gates.
  case "${kind}" in
    gate) grep -q '^## 5 · Rules that bind this gate$' "${file}" \
      || fail "${id}: a gate's section 5 heading must read 'Rules that bind this gate'" ;;
    *) grep -q '^## 5 · Rules that bind this issue$' "${file}" \
      || fail "${id}: section 5 heading must read 'Rules that bind this issue'" ;;
  esac

  grep -qF 'Source of truth: `ROADMAP.md`' "${file}" \
    || fail "${id}: no staleness line naming the roadmap row"

  chars="$(wc -c <"${file}")"
  if [ "${chars}" -lt 2500 ]; then
    fail "${id}: ${chars} characters, under the contract's floor; a short body is usually a placeholder"
  elif [ "${chars}" -gt 6000 ]; then
    fail "${id}: ${chars} characters, over the point where an issue becomes a tutorial"
  fi

  # The organization writes prose without em dashes; the bytes are spelled out so
  # this file does not itself carry one.
  if LC_ALL=C grep -q $'\xe2\x80\x94' "${file}"; then
    fail "${id}: contains an em dash"
  fi
  if awk 'length($0) > 106 { found = 1 } END { exit !found }' "${file}"; then
    fail "${id}: a line runs past 106 columns"
  fi

  case "${kind}" in
    gate)
      grep -qF 'A gate closes on a reading, not on a diff.' "${file}" \
        || fail "${id}: a gate must say it closes on an observation, not a diff"
      ;;
    owner)
      grep -qF 'This row is an act, not a task' "${file}" \
        || fail "${id}: an owner act must say a contributor cannot take it"
      ;;
    task)
      grep -qF 'is ticked in this pull request' "${file}" \
        || fail "${id}: no roadmap-tick box in section 2"
      grep -qF '`date -u`' "${file}" \
        || fail "${id}: the roadmap-tick box does not name the UTC stamp"
      ;;
  esac

  # Every body separates what a desk can prove from what needs the hosted half.
  grep -qF 'Local half:' "${file}" || fail "${id}: no local half in section 6"
  grep -qF 'Hosted half:' "${file}" || fail "${id}: no hosted half in section 6"

  # A wave tier claims the work can start, so the same body cannot also name an
  # unticked dependency as the reason to stop.
  if printf '%s' "${line}" | grep -q ' | wave: '; then
    waved=$((waved + 1))
    if grep -qF 'is not ticked yet' "${file}"; then
      fail "${id}: carries a wave tier while a dependency is unticked"
    fi
  fi

  printf 'ok   %s (%s, %s characters)\n' "${id}" "${kind}" "${chars}"
}

# The same id space the generator publishes: phases 0 to 6 plus the gates.
# Section 7 of the roadmap is the unprioritised backlog and gets no issue.
open_tasks="$(awk '/^- \[[ ]\] \*\*(G[0-9]+|[0-6]\.[0-9]+)\*\*/ {
    id = substr($0, 9); sub(/\*\*.*/, "", id); print id
  }' "${roadmap}")"

if [ -z "${open_tasks}" ]; then
  fail "no open tasks found in ${roadmap}; the reader is broken"
fi

bodies=0
waved=0
while IFS= read -r id; do
  [ -n "${id}" ] || continue
  if ! "${gen}" --body-for "${id}" >"${tmp}/${id}.md" 2>"${tmp}/${id}.err"; then
    fail "${id}: --body-for refused: $(cat "${tmp}/${id}.err")"
    continue
  fi
  check_body "${id}" "${tmp}/${id}.md"
  bodies=$((bodies + 1))
done <<<"${open_tasks}"

# The dependency rule has to clear for at least one task, or a parser that
# quietly reads no ticked row would strip every wave tier and still pass.
if [ "${waved}" -eq 0 ]; then
  fail "no body carries a wave tier: the dependency check reads nothing as done"
else
  printf 'ok   %s body(ies) carry a wave tier\n' "${waved}"
fi

# A task with no curated prose gets no body: the contract forbids a generated
# section 1, and this is the guard that stops a new roadmap row minting an
# empty issue. 0.5 is ticked and was never given prose, so it refuses.
if "${gen}" --body-for 0.5 >/dev/null 2>&1; then
  fail "a task without curated prose produced a body"
else
  printf 'ok   a task with no curated prose is refused\n'
fi

# An id with no backlog row is an error rather than an empty issue.
if "${gen}" --body-for 99.99 2>/dev/null | grep -q .; then
  fail "an unknown id produced output"
else
  printf 'ok   an unknown id is refused\n'
fi

if [ "${failures}" -gt 0 ]; then
  note "test_issue_shape: ${failures} check(s) failed across ${bodies} body(ies)"
  exit 1
fi
note "test_issue_shape: ${bodies} body(ies) match the contract"
