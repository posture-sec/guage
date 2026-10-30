#!/usr/bin/env bash
# Self-test for the roadmap gate scripts. Builds throwaway git repos in a temp
# directory so the cases exercise real diffs, then reports failures.
set -uo pipefail

src_repo="$(git rev-parse --show-toplevel)"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT
failures=0

check() {
  local name="$1" want="$2"
  shift 2
  if "$@" >/dev/null 2>&1; then got=0; else got=1; fi
  if [ "${got}" != "${want}" ]; then
    echo "FAIL ${name}: expected exit ${want}, got ${got}" >&2
    failures=$((failures + 1))
  else
    echo "ok   ${name}"
  fi
}

scaffold() {
  local repo="${tmp}/$1"
  mkdir -p "${repo}/scripts"
  cp "${src_repo}/scripts/check_roadmap_touched.sh" "${src_repo}/scripts/roadmap_progress.sh" "${repo}/scripts/"
  cat > "${repo}/ROADMAP.md" <<'EOF'
Last updated: 2026-01-01T00:00Z

## 2. Progress

<!-- PROGRESS:START -->
| Phase | Name | Done | Total | Progress |
|---|---|---|---|---|
| 0 | Validation and decisions | 1 | 2 | 50% |
| 1 | Repository bootstrap and self-protection | 0 | 1 | 0% |
| 2 | Core | 0 | 0 | 0% |
| 3 | Rules | 0 | 0 | 0% |
| 4 | Dogfood and release v0.1.0 | 0 | 0 | 0% |
| 5 | v0.2 features | 0 | 0 | 0% |
| 6 | Release v0.2.0 and community | 0 | 0 | 0% |
| 7 (backlog) | Backlog | 0 | 0 | 0% |
| **Phases 0 to 6** | **Readiness total** | **1** | **3** | **33%** |
| Gates | G1 to G10 | 0 | 1 | 0% |
<!-- PROGRESS:END -->

### Phase 0

- [x] **0.1** Done thing.
- [ ] **0.2** Pending thing.

### Phase 1

- [ ] **1.1** Pending thing.

## 3. Readiness gates

- [ ] **G1** A gate.
EOF
  ( cd "${repo}" && git -c init.defaultBranch=main init -q \
    && git add ROADMAP.md scripts && git -c user.email=t@example -c user.name=t commit -qm base )
  echo "${repo}"
}

touch_file() { echo change >> "$1/file.txt"; }

# Case 1: code changed, ROADMAP.md untouched -> gate must fail.
repo="$(scaffold c1)"
( cd "${repo}" && git checkout -qb work )
touch_file "${repo}"
( cd "${repo}" && git add file.txt && git -c user.email=t@example -c user.name=t commit -qm change )
check "fails when ROADMAP.md is not touched" 1 env -C "${repo}" scripts/check_roadmap_touched.sh main

# Case 2: ROADMAP.md changed but the Last updated line is identical -> must fail.
repo="$(scaffold c2)"
( cd "${repo}" && git checkout -qb work )
touch_file "${repo}"
printf '\nExtra notes.\n' >> "${repo}/ROADMAP.md"
( cd "${repo}" && git add . && git -c user.email=t@example -c user.name=t commit -qm change )
check "fails when Last updated is unchanged" 1 env -C "${repo}" scripts/check_roadmap_touched.sh main

# Case 3: ROADMAP.md touched and Last updated refreshed -> must pass.
repo="$(scaffold c3)"
( cd "${repo}" && git checkout -qb work )
touch_file "${repo}"
sed -i '1s/.*/Last updated: 2026-01-02T00:00Z/' "${repo}/ROADMAP.md"
( cd "${repo}" && git add . && git -c user.email=t@example -c user.name=t commit -qm change )
check "passes when ROADMAP.md and Last updated change" 0 env -C "${repo}" scripts/check_roadmap_touched.sh main

# Case 4: an empty diff has nothing to enforce.
repo="$(scaffold c4)"
check "passes on an empty diff" 0 env -C "${repo}" scripts/check_roadmap_touched.sh main

# Case 5: a missing base ref is an error, not a silent pass.
repo="$(scaffold c5)"
check "fails when the base ref is missing" 1 env -C "${repo}" scripts/check_roadmap_touched.sh nope

# Case 6: the progress script recomputes the committed table exactly.
repo="$(scaffold c6)"
check "progress table matches the tasks" 0 env -C "${repo}" scripts/roadmap_progress.sh --check ROADMAP.md

# Case 7: ticking a task without refreshing the table must fail.
repo="$(scaffold c7)"
sed -i 's/^- \[ \] \*\*1.1\*\*/- [x] **1.1**/' "${repo}/ROADMAP.md"
check "progress table is stale after a tick" 1 env -C "${repo}" scripts/roadmap_progress.sh --check ROADMAP.md

# Case 8: computed numbers land in the right rows, including the readiness total.
repo="$(scaffold c8)"
row="$( cd "${repo}" && scripts/roadmap_progress.sh | grep 'Phases 0 to 6' )"
if [ "${row}" = "| **Phases 0 to 6** | **Readiness total** | **1** | **3** | **33%** |" ]; then
  echo "ok   readiness total computed"
else
  echo "FAIL readiness total computed: ${row}" >&2
  failures=$((failures + 1))
fi

if [ "${failures}" -gt 0 ]; then
  echo "test_roadmap_gate: ${failures} case(s) failed" >&2
  exit 1
fi
echo "test_roadmap_gate: all cases passed"
