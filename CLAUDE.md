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
- `internal/checkrun` — `rg check` pipeline (load snapshot, rerun, diff, render).
- `internal/snapshot` — baseline read/write + HMAC integrity + field redaction.
- `internal/mcprun` — exposes snapshot/check/status as MCP tools (strategic differentiator).
- `internal/ui` — design system; all color via `ui.Paint()`, animations TTY-only.

## Diff severity (the scoring logic)
- CRITICAL: new test failures, status-code change, schema-hash mismatch, route missing.
- WARNING: timing increase `>200ms AND >50%` of baseline.
- PASS: within variance. Exit codes: 0 pass/warn · 1 critical · 2 error.
Dynamic keys (16) + ISO-8601/UUID/JWT patterns are stripped before hashing to prevent
false positives — this is the core trust mechanism; do not weaken it without tests.

## Working agreement
- Single task in scope at a time (AGENTS.md workflow). Verify acceptance criteria before
  marking done. `go test ./...` must stay green; gate slow tests under `-short`.
- Do not expand stack support (Python/FastAPI/Django), add dashboards, visual regression,
  or AI-generated tests without updating PRD change-control first.
