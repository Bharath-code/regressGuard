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

### P0-2 · Distinguish config-removal vs errored — ⏸️ DEFERRED (2026-06-15)
The errored-vs-skip distinction is delivered by P0-1 (errored → `Unverified` → WARNING).
The remaining piece — downgrading a *genuine* config-removal from CRITICAL to
"informational only" — is **deferred on review**: removing a route from config arguably
*should* still warn, and the change would intentionally break the existing
`TestDiffSnapshots_routeDisappeared_critical` contract for little user benefit. Revisit
only if a real user reports a false block from an intentional route removal.

### P0-3 · Secret hygiene for `auth.testToken` — ✅ ALREADY IMPLEMENTED (verified 2026-06-15)
Review correction: this risk was overstated. The capability already exists and is wired:
- [x] Env-var indirection: `config.resolveEnv` resolves `$VAR`/`${VAR}` in `testToken` and
      `cookie` at load (`config.go:84`), plus `.regressguard/.env` auto-load.
- [x] `rg init` never persists a literal token — it only sets auth mode + header
      (`initrun.go:114`); no token is written to `config.json`.
- [x] `rg doctor` warns on raw secrets (`config.LooksLikeSecret`), unsafe `.env`
      permissions, and token age >30 days (`doctorrun.go:99-119`).
- [x] `.regressguard/` (incl. `config.json`) is git-ignored by default (`.gitignore`).
- [ ] Residual gap (low priority): `doctorrun` and `initrun` have no unit tests for the
      secret-warning path. Track under test-coverage backlog, not a P0.

### P1-1 · Calm-by-default UX (animations opt-in) — ✅ DONE (2026-06-16)
**Implementation:** added a package-level `ui.SetAnimations`/`ui.AnimationsEnabled` toggle
(default OFF). The five motion primitives (`StaggeredPrint`, `AnimatedTableRow`,
`SuccessCelebration`, `CriticalReveal`, and the live `RouteProgress` table) now gate their
timing on `animationsEnabled && ColorEnabled(w)` — styling/color is preserved, only the
sleeps/frames are dropped. The `Spinner` defers its first frame by `spinnerStartDelay`
(400ms) so quick phases never flash. `rg check`/`rg snapshot` call `SetAnimations(opts.Celebrate)`
at startup; new `--celebrate` flag on both commands opts back in.
**Acceptance criteria:**
- [x] Default `rg check`/`rg snapshot` output renders instantly: no per-row slide-in, no
      celebration, no staggered reveal. Spinners remain only for operations >400ms
      (deferred first frame).
- [x] Animations available behind `--celebrate` flag.
- [x] Non-TTY/JSON/hook/CI behavior unchanged (gated on `ColorEnabled`, already animation-free).
- [x] Output still fits one 80-column viewport (`TestCommandHelpIncludesContractSections`
      green) and existing `ui` tests pass.
- [x] New tests (`internal/ui/animation_test.go`): celebration/reveal/stagger render
      instantly with animations off and animate when on; fast operation shows no spinner frame.

### P1-2 · Test + harden the MCP path (the wedge) — ✅ DONE (2026-06-15)
**Implementation:** added `internal/mcprun/mcprun_test.go`; hardened `validatePath` (S4)
to use `filepath.Rel` instead of a `strings.HasPrefix` compare that wrongly accepted a
sibling like `/root-evil` for project root `/root`.
**Acceptance criteria:**
- [x] Tests for the check tool (happy path + missing-config error), status tool (happy
      path), audit-log write (single + appended entries), and `Serve` root validation.
- [x] MCP tool responses return the same structured JSON as `--json` (check handler test
      parses the content back into the `status`/`summary` shape).
- [x] Audit log entries are append-only and include tool, status, durationMs, timestamp
      (asserted).
- [x] S4 path-traversal guard hardened + regression-tested (sibling-prefix case).
- [ ] Follow-up (not blocking): `validatePath` is a security primitive not yet wired to a
      live tool arg (current tools take a git ref, not a path). Wire it when a path-taking
      tool is added, or remove it. One-line Claude Code / Cursor registration doc still TODO.

### P1-3 · Document scoring semantics + close `passedRoutes` cosmetic gap — ✅ DONE (2026-06-16)
**Implementation:** `diff.go` now tracks `routeWarning` alongside `routeCritical`; a route is
counted in `PassedRoutes` only when it produced no finding at all, so a warning-only route
(e.g. timing) is excluded from "Routes: N unchanged" and from `summary.passed` in `--json`
(the agent/MCP contract). README "How it works" gained a **Known limitations** section.
**Acceptance criteria:**
- [x] README "How it works" documents the test-failure-delta limitation (failures compared
      by count, not identity) and the array first-element comparison limitation.
- [x] A route with only a timing WARNING is reported separately, not counted in
      "Routes: N unchanged."
- [x] Test asserts the count excludes warning-only routes
      (`TestDiffSnapshots_timingWarningRoute_notCountedAsPassed`).

### P2-1 · Open-core monetization stub / positioning — ✅ DONE (2026-06-16)
**Implementation (docs only — no code):** README re-centered on agent-native verification
(MCP as the primary mode; new "Agent-native verification (MCP)" section with `rg mcp serve`
+ Claude Code/Cursor registration; `rg mcp serve` added to the command table; human CLI/CI
framed as secondary). Added `docs/json-contract.md` (stable, versioned `rg check --json`
contract for agents + future SaaS ingest) and `docs/paid-layer-spec.md` (one-page open-core
boundary + org dashboard / history retention / compliance export, scoped not built). README
gained an "Open core" footer linking the paid spec.
**Acceptance criteria:**
- [x] README + landing narrative re-centered on "agent-native verification" (RG in the
      agent loop), with the human CI use-case secondary.
- [x] A documented `rg check --json` contract suitable for a future SaaS ingest
      (`docs/json-contract.md`).
- [x] One-page spec for the paid layer (org dashboard, history retention, compliance
      export) committed to `docs/` — not built, just scoped (`docs/paid-layer-spec.md`).

### P2-2 · Expand wedge surface (stacks)
**Acceptance criteria:**
- [ ] Spec (not impl) for FastAPI/Django route discovery in `scanner`, gated behind PRD
      change-control per AGENTS.md scope rules.

---

## Out of scope (per AGENTS.md until PRD change-control updated)
Dashboards, Python support, visual regression, AI-generated tests, enterprise compliance
features — P2 items above are **specs only**, not implementation.
