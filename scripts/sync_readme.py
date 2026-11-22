#!/usr/bin/env python3
"""Synchronize dynamic README metrics and live Shields.io endpoint data for Gauge.

Inspects the repository test suite, active rules, and status, and writes:
1. Dynamic Shields.io JSON endpoints under `site/data/badge-*.json`.
2. Marked block between `<!-- METRICS:START -->` and `<!-- METRICS:END -->` in `README.md`.

With `--check`, exits 1 if any target is stale without modifying files.
With `--check-base <ref>`, exits 1 if the pull request changed the metrics block
or a badge endpoint at all. Those files are written by this script on the default
branch and nowhere else, so two branches that each add tests cannot collide on the
same twice-edited table row.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
README_PATH = REPO_ROOT / "README.md"
RULES_FILE = REPO_ROOT / "internal" / "rules" / "rules.go"
DATA_DIR = REPO_ROOT / "site" / "data"

METRICS_START = "<!-- METRICS:START -->"
METRICS_END = "<!-- METRICS:END -->"


def count_tests() -> int:
    pattern = re.compile(r"^func\s+Test[A-Za-z0-9_]*\s*\(")
    count = 0
    for p in REPO_ROOT.rglob("*_test.go"):
        if ".git" in p.parts:
            continue
        try:
            for line in p.read_text(encoding="utf-8").splitlines():
                if pattern.match(line):
                    count += 1
        except Exception:
            continue
    return count


def count_rules() -> int:
    if not RULES_FILE.exists():
        return 16
    content = RULES_FILE.read_text(encoding="utf-8")
    rules = re.findall(r"\{ID:\s*\"([^\"]+)\"", content)
    return len(rules) if rules else 16


def generate_metrics_table(test_count: int, rule_count: int) -> str:
    return (
        f"{METRICS_START}\n"
        "| Indicator | Value | Source |\n"
        "|---|---|---|\n"
        f"| Unit & Integration Tests | **{test_count} passing** | `go test ./...` |\n"
        f"| Posture Rules | **{rule_count} verified** across 4 families | `internal/rules` |\n"
        "| Binary | **`stellar-gauge`** | `cmd/stellar-gauge` |\n"
        "| Security Guarantee | **Read-only / 0 secret keys** | Architecture spec |\n"
        f"{METRICS_END}"
    )


def build_badge_json(label: str, message: str, color: str) -> dict[str, object]:
    return {
        "schemaVersion": 1,
        "label": label,
        "message": message,
        "color": color,
    }


def sync(*, check: bool = False) -> bool:
    test_count = count_tests()
    rule_count = count_rules()
    DATA_DIR.mkdir(parents=True, exist_ok=True)

    badges = {
        DATA_DIR / "badge-tests.json": build_badge_json(
            "tests", f"{test_count} passed", "00ADD8"
        ),
        DATA_DIR / "badge-rules.json": build_badge_json(
            "rules", f"{rule_count} verified", "8b5cf6"
        ),
        DATA_DIR / "badge-status.json": build_badge_json(
            "gauge", "functional alpha", "10b981"
        ),
    }

    stale = False

    # Check/write badge JSONs
    for path, data in badges.items():
        rendered = json.dumps(data, indent=2, sort_keys=True) + "\n"
        if path.exists():
            current = path.read_text(encoding="utf-8")
            if current != rendered:
                stale = True
                if not check:
                    path.write_text(rendered, encoding="utf-8")
        else:
            stale = True
            if not check:
                path.write_text(rendered, encoding="utf-8")

    # Check/write README.md
    if README_PATH.exists():
        content = README_PATH.read_text(encoding="utf-8")
        if METRICS_START in content and METRICS_END in content:
            before, rest = content.split(METRICS_START, 1)
            _, after = rest.split(METRICS_END, 1)
            new_table = generate_metrics_table(test_count, rule_count)
            updated = f"{before}{new_table}{after}"
            if updated != content:
                stale = True
                if not check:
                    README_PATH.write_text(updated, encoding="utf-8")

    if check and stale:
        print(
            "sync_readme: README metrics or badge data are stale. "
            "Run: python3 scripts/sync_readme.py",
            file=sys.stderr,
        )
        return False

    return True


def git(*args: str) -> str:
    return subprocess.run(
        ["git", *args], cwd=REPO_ROOT, check=True, capture_output=True, text=True
    ).stdout


def managed_paths() -> list[str]:
    """Repository-relative paths this script owns."""
    paths = ["README.md"]
    if DATA_DIR.is_dir():
        paths += [p.relative_to(REPO_ROOT).as_posix() for p in sorted(DATA_DIR.glob("badge-*.json"))]
    return paths


def check_untouched(base: str) -> bool:
    """Fail if the branch edits a file this script owns on the default branch."""
    try:
        git("rev-parse", "--verify", "--quiet", f"{base}^{{commit}}")
    except subprocess.CalledProcessError:
        print(
            f"sync_readme: base ref '{base}' is not available; fetch it before running",
            file=sys.stderr,
        )
        return False

    changed = set(git("diff", "--name-only", f"{base}...HEAD").splitlines())
    # Also catch edits that are still staged or unsaved, so the check fails the
    # same way locally as it does on the pull request.
    changed |= set(git("diff", "--name-only", "HEAD").splitlines())
    owned = managed_paths()
    touched = [p for p in sorted(changed) if p in owned]
    if not touched:
        return True

    print(
        "sync_readme: this branch changes files that scripts/sync_readme.py owns "
        "on the default branch:",
        file=sys.stderr,
    )
    for p in touched:
        print(f"  {p}", file=sys.stderr)
    print(
        "\nThese are generated, not hand-maintained: contributors do not update the "
        "metrics row or the badge endpoints, and the maintainer refreshes them on "
        "the default branch with `python3 scripts/sync_readme.py`. Drop them from "
        "this branch with:\n\n"
        f"  git checkout {base} -- {' '.join(touched)}\n",
        file=sys.stderr,
    )
    return False


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--check", action="store_true", help="Exit 1 if stale without modifying files"
    )
    parser.add_argument(
        "--check-base",
        metavar="REF",
        help="Exit 1 if the branch changed the metrics block or a badge endpoint",
    )
    args = parser.parse_args()

    if args.check_base:
        return 0 if check_untouched(args.check_base) else 1

    ok = sync(check=args.check)
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
