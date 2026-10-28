# Contributing to Gauge

Status: process draft. The tool is a scaffold (ROADMAP 1.12, D-09) and feature work is gated by the unfinished Phase 0 tasks.

Read in order: `ROADMAP.md` section 4, `ARCHITECTURE_ESSENTIALS.md`, `project_structure.md`, then the cited section of `ARCHITECTURE.md` or `PRD.md`.

## Workflow

1. Find the task in `ROADMAP.md`. If it does not exist, add it with the next free ID in its phase and say why.
2. Confirm its dependencies are done. If not, stop and report.
3. Write the failing test or fixture first, then the smallest change that passes.
4. Branch `feat/<task-id>-<slug>`. One task or one tightly related group per pull request. Never commit to the default branch.
5. Use Conventional Commits with a subject under 72 characters.
6. Update the docs the change touches, and `project_structure.md` for any added, moved or removed file.
7. Update `ROADMAP.md` in the same pull request: task boxes, changelog row (UTC date, summary, task IDs), progress table, `Last updated`.
8. Do not add a Go module, loosen a check in a protected path, or start a network call without owner approval.

Checks in `AGENTS.md` section 5 run once they exist; state which are missing rather than skipping them silently.
