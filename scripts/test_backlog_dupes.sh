#!/usr/bin/env bash
# Proves generate_issues.sh refuses a duplicated backlog id in --check mode,
# while the unmodified script still reports that the backlog is complete.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

stub_dir="${tmp}/bin"
mkdir -p "${stub_dir}"
printf '#!/usr/bin/env bash\nexit 99\n' >"${stub_dir}/gh"
chmod +x "${stub_dir}/gh"
export PATH="${stub_dir}:${PATH}"

script="${repo_root}/scripts/generate_issues.sh"
duplicate_copy="${tmp}/generate_issues.sh"
duplicate_id="1.9"

awk '
  !done && /enforce the roadmap rule with a required gate check/ { print; print; done = 1; next }
  { print }
' "${script}" >"${duplicate_copy}"

cd "${repo_root}"

if stderr="$(bash "${duplicate_copy}" --check 2>&1 >/dev/null)"; then
  code=0
else
  code=$?
fi
if [ "${code}" -ne 1 ]; then
  echo "test_backlog_dupes: duplicate copy exited ${code}, want 1" >&2
  exit 1
fi
want="generate_issues: duplicate backlog row for ${duplicate_id}"
if ! printf '%s\n' "${stderr}" | grep -qxF "${want}"; then
  echo "test_backlog_dupes: duplicate copy stderr did not carry: ${want}" >&2
  printf '%s\n' "${stderr}" >&2
  exit 1
fi

if output="$(bash "${script}" --check 2>&1)"; then
  code=0
else
  code=$?
fi
if [ "${code}" -ne 0 ]; then
  echo "test_backlog_dupes: unmodified script exited ${code}, want 0" >&2
  printf '%s\n' "${output}" >&2
  exit 1
fi
want="generate_issues: backlog covers every open task"
if ! printf '%s\n' "${output}" | grep -qF "${want}"; then
  echo "test_backlog_dupes: unmodified script did not carry: ${want}" >&2
  printf '%s\n' "${output}" >&2
  exit 1
fi

echo "test_backlog_dupes: duplicated row refused, unmodified backlog complete"
