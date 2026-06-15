# RegressGuard — Staff/Principal + C-Suite Review & Go-Forward Plan

_Date: 2026-06-15 · Reviewer role: Senior Staff / Principal Eng + CEO/CFO/CMO/CTO/AI-PM lenses_

## Verdict: PURSUE (conditional)

The problem is real and well-evidenced (AI-code verification gap). Build quality is high —
clean idiomatic Go, sound architecture, all test packages passing, thoughtful security
(HMAC integrity, redaction, MCP audit log). The defensible wedge is **agent-native
verification via MCP** (RG as a tool inside the AI agent's own loop), which no incumbent
(Touca, Jest snapshots, Keploy, Optic, Pact) owns.

Three conditions gate any growth spend:

1. Eliminate the transient-error false positive (trust is the entire product).
2. Make the daily CLI experience calm (animations opt-in, instant default).
3. Sharpen positioning + harden the MCP path; make the open-core/monetization story real.

---

## Findings summary

### Engineering
- Build clean; `engine`, `checkrun`, `config`, `history`, `scanner`, `snapshotrun`, `ui`,
  `cli`, `explainrun`, `hookrun`, `initrun` test packages all pass.
- Scoring/diff logic audited (see task P0-1). Timing gate (`>200ms AND >50%`) and
  schema-hash guards are correct. Dynamic-key normalization is the core FP defense and is
  well tested.
- Test gaps: `snapshot`, `mcprun`, `watchrun`, `statusrun`, `doctorrun` have no tests.
  The MCP server (the strategic differentiator) is untested.

### Security
- `auth.testToken` stored plaintext in `config.json`, which is not git-ignored by default.

### UX / friction
- `rg check` animates on every run (staggered reveal, 60ms/row table slide-in, reveals,
  celebration). For a tool run dozens of times/day this reads as jarring, not elegant.
- Friction already low (`quickstart`, `watch`, `hook`, `--auto-server`, MCP). MCP is the
  best "fewer steps" lever — agent self-verifies, zero human steps.

### Business
- Open-core: this repo is the free CLI only. The revenue layer (enterprise dashboard,
  compliance, retention) is unbuilt. Willingness-to-pay lives at team/org layer, not the
  individual CLI user.

---

## Task breakdown (with acceptance criteria)

Priorities: **P0** = trust/correctness blockers · **P1** = polish + wedge · **P2** = growth/monetization.

### P0-1 · Fix transient-error false positive (route missing in `after`) — ✅ DONE (2026-06-15)
**Problem:** `checkrun.go` drops skipped routes from the after-snapshot; `diff.go` then flags
any baseline route absent in `after` as CRITICAL "route no longer present." A single
transient timeout / 5xx / momentary unreachable during `rg check` blocks the commit.
**Implementation:** new `RouteResult.Errored` flag distinguishes transient failures from
intentional skips; errored routes are recorded as `snapshot.RouteRecord{Unverified:true}`;
`diff.go` reports `TypeUnverified` as a non-blocking WARNING and skips status/schema/timing
comparison; auto-refresh never persists Unverified records into a baseline.
**Acceptance criteria:**
- [x] A route that is `Skipped` for a transient reason (timeout/connection error) during
      `rg check` does NOT produce a CRITICAL finding.
- [x] Surfaced as a non-blocking WARNING (`TypeUnverified`, "could not verify … — not
      counted as a regression"); exit code unaffected (warning → 0). _Note: emitted as a
      structured finding (warning screen + `--json` Results) rather than only stderr — better
      for agent/MCP consumption._
- [x] A route genuinely removed/deleted still behaves as today
      (`TestDiffSnapshots_routeDisappeared_critical` still passes); intentional config-skips
      remain omitted.
- [x] New tests: `TestDiffSnapshots_unverifiedRoute_warningNotCritical`,
      `TestDiffSnapshots_unverifiedRoute_notCountedAsPassed` (engine) and
      `TestRun_transientRouteError_notCritical` (checkrun, hijack-drops-connection harness).
- [x] `go test ./...` green; `go build ./...` clean.

### P0-2 · Distinguish "route absent because removed from config" vs "absent because errored"
**Acceptance criteria:**
- [ ] `afterSnap` retains errored routes with an explicit `Unverified` marker instead of
      omitting them, so the diff engine can tell the two cases apart.
- [ ] Diff engine treats `Unverified` as WARNING, true config-removal as informational only.
- [ ] Unit test covers both branches.

### P0-3 · Secret hygiene for `auth.testToken`
**Acceptance criteria:**
- [ ] `config.json` supports env-var indirection: `"testToken": "${RG_TEST_TOKEN}"`
      resolved at load time.
- [ ] `rg init` writes the env-var form by default and never persists a literal token it
      was given interactively without confirmation.
- [ ] `rg doctor` warns if a literal secret-looking value is present in a git-tracked
      `config.json`.
- [ ] `rg init` adds `.regressguard/config.json` to `.gitignore` (or documents why not).
- [ ] Tests for env resolution + doctor warning.

### P1-1 · Calm-by-default UX (animations opt-in)
**Acceptance criteria:**
- [ ] Default `rg check`/`rg snapshot` output renders instantly: no per-row slide-in, no
      celebration, no staggered reveal. Spinners remain only for operations >400ms.
- [ ] Animations available behind `--celebrate` flag and/or first-run only.
- [ ] Non-TTY/JSON/hook/CI behavior unchanged (already animation-free).
- [ ] Output still fits one 80-column viewport and passes existing `ui` tests.

### P1-2 · Test + harden the MCP path (the wedge)
**Acceptance criteria:**
- [ ] `internal/mcprun` has tests covering each exposed tool (snapshot, check, status):
      happy path, missing-config error, and audit-log write.
- [ ] MCP tool responses return structured JSON identical in meaning to `--json` CLI output.
- [ ] Documented one-line setup for Claude Code / Cursor to register `rg mcp serve`.
- [ ] Audit log entries are append-only and include tool, status, duration.

### P1-3 · Document scoring semantics + close `passedRoutes` cosmetic gap
**Acceptance criteria:**
- [ ] README "How it works" documents the test-failure-delta limitation and array
      first-element comparison limitation.
- [ ] A route with only a timing WARNING is reported separately, not counted in
      "Routes: N unchanged."
- [ ] Test asserts the count excludes warning-only routes.

### P2-1 · Open-core monetization stub / positioning
**Acceptance criteria:**
- [ ] README + landing narrative re-centered on "agent-native verification" (RG in the
      agent loop), with the human CI use-case secondary.
- [ ] A documented `rg check --json` contract suitable for a future SaaS ingest.
- [ ] One-page spec for the paid layer (org dashboard, history retention, compliance
      export) committed to `docs/` — not built, just scoped, so open-core is credible.

### P2-2 · Expand wedge surface (stacks)
**Acceptance criteria:**
- [ ] Spec (not impl) for FastAPI/Django route discovery in `scanner`, gated behind PRD
      change-control per AGENTS.md scope rules.

---

## Out of scope (per AGENTS.md until PRD change-control updated)
Dashboards, Python support, visual regression, AI-generated tests, enterprise compliance
features — P2 items above are **specs only**, not implementation.
