# CLAUDE.md — RegressGuard

> One-line: a single-binary Go CLI that records a known-good API baseline and blocks
> commits when an AI agent silently regresses it. Also an MCP server so the agent can
> verify its own work.

## Read first
- `AGENTS.md` — workflow, tech stack, project structure, CLI/UX/error/test conventions.
- `RegressGuard-PRD.md` Section 11 — implementation source of truth (task tracker).
- `docs/staff-review-2026-06.md` — current product review, verdict, and prioritized
  task breakdown with acceptance criteria.

`AGENTS.md` rules apply in full; this file does not restate them. On conflict, the PRD
and AGENTS.md win.

## Product verdict (2026-06-15): PURSUE (conditional)
Real, well-evidenced problem; high build quality; unique wedge = **agent-native
verification via MCP**. Gated on three conditions before growth spend:
1. Eliminate transient-error false positives (trust is the whole product).
2. Calm-by-default UX (animations opt-in, instant default).
3. Harden + test the MCP path; make the open-core/monetization story real.

## Current priorities (see docs/staff-review-2026-06.md for acceptance criteria)
- ✅ Done: P0 transient-error FP, P0 secret hygiene (verified), P1-2 MCP hardening + tests,
  P1-1 calm-by-default UX (animations opt-in via `--celebrate`), P1-3 scoring-semantics docs
  + warning-only routes excluded from "unchanged" count, P2-1 open-core positioning +
  `docs/json-contract.md` + `docs/paid-layer-spec.md`.
- **P2-2** (spec only, per AGENTS.md scope) — FastAPI/Django route-discovery spec for
  `scanner`, gated behind PRD change-control before any code.

## Architecture quick map
- `internal/engine` — test runner, route hitter, schema normalizer, diff (severity rules).
- `internal/checkrun` — `regressguard check` pipeline (load snapshot, rerun, diff, render).
- `internal/snapshot` — baseline read/write + field redaction.
- `internal/mcprun` — exposes snapshot/check/status as MCP tools (strategic differentiator).
- `internal/ui` — design system; all color via `ui.Paint()`, animations TTY-only.

## Diff severity (the scoring logic)
- CRITICAL: new test failures, status-code change, schema-hash mismatch, route missing.
- WARNING: timing increase `>200ms AND >50%` of baseline.
- PASS: within variance. Exit codes: 0 pass/warn · 1 critical · 2 error.
Dynamic keys (16) + ISO-8601/UUID/JWT patterns are stripped before hashing to prevent
false positives — this is the core trust mechanism; do not weaken it without tests.

## Releases
Published on the official MCP registry as `io.github.Bharath-code/regressguard` (v0.1.0,
2026-07-17). Every release must also rebuild/upload the `.mcpb` bundle and republish —
follow "Release & MCP Registry Publish" in `AGENTS.md`.

## Working agreement
- Single task in scope at a time (AGENTS.md workflow). Verify acceptance criteria before
  marking done. `go test ./...` must stay green; gate slow tests under `-short`.
- Do not expand stack support (Python/FastAPI/Django), add dashboards, visual regression,
  or AI-generated tests without updating PRD change-control first.

## Workflow Orchestration

### 1. Plan Mode Default
- Enter plan mode for ANY non-trivial task (3+ steps or architectural decisions)
- If something goes sideways, STOP and re-plan immediately – don't keep pushing
- Use plan mode for verification steps, not just building
- Write detailed specs upfront to reduce ambiguity

### 2. Subagent Strategy
- Use subagents liberally to keep main context window clean
- Offload research, exploration, and parallel analysis to subagents
- For complex problems, throw more compute at it via subagents
- One task per subagent for focused execution

### 3. Self-Improvement Loop
- After ANY correction from the user: update `tasks/lessons.md` with the pattern
- Write rules for yourself that prevent the same mistake
- Ruthlessly iterate on these lessons until mistake rate drops
- Review lessons at session start for relevant project

### 4. Verification Before Done
- Never mark a task complete without proving it works
- Diff behavior between main and your changes when relevant
- Ask yourself: "Would a staff engineer approve this?"
- Run tests, check logs, demonstrate correctness

### 5. Demand Elegance (Balanced)
- For non-trivial changes: pause and ask "is there a more elegant way?"
- If a fix feels hacky: "Knowing everything I know now, implement the elegant solution"
- Skip this for simple, obvious fixes – don't over-engineer
- Challenge your own work before presenting it

### 6. Autonomous Bug Fixing
- When given a bug report: just fix it. Don't ask for hand-holding
- Point at logs, errors, failing tests – then resolve them
- Zero context switching required from the user
- Go fix failing CI tests without being told how

## Task Management

1. **Plan First**: Write plan to `tasks/todo.md` with checkable items
2. **Verify Plan**: Check in before starting implementation
3. **Track Progress**: Mark items complete as you go
4. **Explain Changes**: High-level summary at each step
5. **Document Results**: Add review section to `tasks/todo.md`
6. **Capture Lessons**: Update `tasks/lessons.md` after corrections

## Core Principles

- **Simplicity First**: Make every change as simple as possible. Impact minimal code.
- **No Laziness**: Find root causes. No temporary fixes. Senior developer standards.
- **Minimal Impact**: Changes should only touch what's necessary. Avoid introducing bugs.
