# AGENTS.md

## Project Source of Truth

Read `RegressGuard-PRD.md` before making changes.

The PRD is the product, UX, architecture, and delivery tracker. Section 11 is the implementation source of truth.

## Required Workflow

1. Pick a task from Section 11.
2. Change its status to `IN PROGRESS`.
3. Implement only that task’s scope.
4. Verify acceptance criteria.
5. Record evidence in the PRD or implementation notes.
6. Move status to `DONE` only when the Definition of Done is satisfied.

## CLI Contracts

- `--json` writes valid JSON to stdout and nothing else.
- Progress, warnings, verbose logs, and diagnostics go to stderr.
- Non-TTY mode must never prompt or hang.
- Respect `NO_COLOR`, `FORCE_COLOR`, and `TERM=dumb`.
- Exit codes:
  - `0`: pass or warnings only
  - `1`: critical regression
  - `2`: usage/config/runtime error

## UX Rules

- Follow Section 6 terminal design system.
- Default screens should fit in one 80-column terminal viewport.
- Every failure must include a clear next command.
- Do not use emoji.
- Do not print large route lists or raw responses unless requested with `--verbose`.

## Scope Control

v1 is focused on Next.js/TypeScript API regression safety.

Do not add:
- dashboards
- Python support
- visual regression testing
- AI-generated tests
- enterprise compliance features

unless the PRD change-control section is updated first.