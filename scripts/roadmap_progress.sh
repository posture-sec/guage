#!/usr/bin/env bash
# Computes the ROADMAP.md progress table from the task checkboxes.
#
# Usage: scripts/roadmap_progress.sh [--check] [roadmap-file]
#   default: print the table
#   --check: exit 1 when the committed table between the PROGRESS markers is stale
set -euo pipefail

mode="print"
if [ "${1:-}" = "--check" ]; then
  mode="check"
  shift
fi
file="${1:-ROADMAP.md}"

if [ ! -f "${file}" ]; then
  echo "roadmap_progress: ${file} not found" >&2
  exit 1
fi

table="$(awk '
  /^- \[[ xX]\] \*\*[0-9]+\.[0-9]+\*\*/ {
    split(substr($0, 9), parts, ".")
    phase = parts[1] + 0
    total[phase]++
    if (substr($0, 4, 1) != " ") done[phase]++
    next
  }
  /^- \[[ xX]\] \*\*G[0-9]+\*\*/ {
    gates_total++
    if (substr($0, 4, 1) != " ") gates_done++
    next
  }
  END {
    names[0] = "Validation and decisions"
    names[1] = "Repository bootstrap and self-protection"
    names[2] = "Core"
    names[3] = "Rules"
    names[4] = "Dogfood and release v0.1.0"
    names[5] = "v0.2 features"
    names[6] = "Release v0.2.0 and community"
    names[7] = "Backlog"
    print "| Phase | Name | Done | Total | Progress |"
    print "|---|---|---|---|---|"
    ready_done = 0
    ready_total = 0
    for (p = 0; p <= 7; p++) {
      d = done[p] + 0
      t = total[p] + 0
      label = (p == 7 ? "7 (backlog)" : p)
      if (p < 7) { ready_done += d; ready_total += t }
      printf "| %s | %s | %d | %d | %d%% |\n", label, names[p], d, t, (t ? int(d * 100 / t) : 0)
    }
    printf "| **Phases 0 to 6** | **Readiness total** | **%d** | **%d** | **%d%%** |\n", \
      ready_done, ready_total, (ready_total ? int(ready_done * 100 / ready_total) : 0)
    printf "| Gates | G1 to G10 | %d | %d | %d%% |\n", \
      gates_done + 0, gates_total + 0, (gates_total ? int((gates_done + 0) * 100 / gates_total) : 0)
  }
' "${file}")"

if [ "${mode}" = "print" ]; then
  printf '%s\n' "${table}"
  exit 0
fi

current="$(awk '/<!-- PROGRESS:START -->/{flag=1;next} /<!-- PROGRESS:END -->/{flag=0} flag' "${file}" \
  | sed '/^[[:space:]]*$/d')"

if [ "${current}" = "${table}" ]; then
  echo "roadmap_progress: table is current"
  exit 0
fi

echo "roadmap_progress: the progress table in ${file} is stale. Expected:" >&2
printf '%s\n' "${table}" >&2
echo "Found:" >&2
printf '%s\n' "${current}" >&2
exit 1
