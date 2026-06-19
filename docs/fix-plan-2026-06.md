# Fix Plan — Functional Accuracy Audit

**Date:** 2026-06-19
**Source:** `docs/functional-accuracy-audit-2026-06.md`
**Status:** All fixes complete — verified 2026-06-19

---

## Fix 1: rg/ripgrep name conflict in git hooks (BUG-1)

**Problem:** The git hook script runs `RG_HOOK=1 rg check`. On machines with ripgrep installed (the `rg` command), this runs ripgrep instead of RegressGuard. The safety net is silently bypassed.

**Root cause:** `internal/hookrun/hookrun.go:24` — `hookScript` uses bare `rg` which resolves to whatever is first in PATH (often ripgrep).

**Fix:** The hook script must resolve the RegressGuard binary path at install time and use the absolute path. Fallback to `regressguard` if the binary can be found, then `rg` with a conflict-detection comment.

**Acceptance criteria:**
- [x] Hook script uses the absolute path of the RegressGuard binary at install time
- [x] If the binary path cannot be resolved, the hook uses `regressguard` (the long name)
- [x] `rg hook install` detects ripgrep conflict and warns the user
- [x] Existing tests updated: `TestInstall_createsHookFile`, `TestInstall_hookScriptBlocksOnExit1`, `TestInstall_outputMentionsHookPath`
- [x] New test: `TestInstall_hookScriptUsesAbsolutePath` — verifies the hook contains the resolved binary path
- [x] New test: `TestInstall_warnsOnRipgrepConflict` — verifies a warning is printed when `rg` resolves to ripgrep
- [x] `go test ./internal/hookrun/...` passes
- [x] `go build ./...` succeeds

---

## Fix 2: Field additions should be WARNING not CRITICAL (BUG-2)

**Problem:** When a new field is added to an API response (backward-compatible change), `diff.go` treats ALL schema hash mismatches as CRITICAL. PRD §8.2 says "optional field added" = WARNING.

**Root cause:** `internal/engine/diff.go:130-142` — severity is hardcoded to `SeverityCritical` for all schema changes.

**Fix:** Add a `classifySchemaChange` function that examines the `FieldChanges` array. If ALL changes are "added", severity is WARNING. If ANY change is "removed" or "changed", severity is CRITICAL.

**Acceptance criteria:**
- [x] `classifySchemaChange([]FieldChange) string` function added to `diff.go`
- [x] All-added → WARNING; any removed/changed → CRITICAL
- [x] `diff.go` uses `classifySchemaChange` to set schema finding severity
- [x] WARNING-only schema routes do not block commits (exit 0)
- [x] WARNING-only schema routes are not counted in `PassedRoutes`
- [x] Existing tests updated: `TestDiffSnapshots_schemaChange_critical` (now tests removal = critical)
- [x] New test: `TestDiffSnapshots_schemaFieldAdded_warning` — field added = WARNING, exit 0
- [x] New test: `TestDiffSnapshots_schemaFieldRemoved_critical` — field removed = CRITICAL
- [x] New test: `TestDiffSnapshots_schemaMixedChanges_critical` — added + removed = CRITICAL
- [x] New test: `TestDiffSnapshots_schemaAddedRoute_notCountedAsPassed`
- [x] `go test ./internal/engine/...` passes

---

## Fix 3: Fix durationMs serialization (BUG-3)

**Problem:** `snapshot.TestSummary.Duration` is `time.Duration` (nanoseconds) with JSON tag `durationMs`. Go serializes `time.Duration` as raw nanosecond integer. The field says "milliseconds" but stores nanoseconds.

**Root cause:** `internal/snapshot/snapshot.go:35` — `Duration time.Duration` with JSON tag `durationMs`.

**Fix:** Change `Duration` to `DurationMs int64` and convert `time.Duration` to milliseconds at every call site.

**Acceptance criteria:**
- [x] `snapshot.TestSummary.Duration` field changed to `DurationMs int64`
- [x] All call sites updated to use `.Milliseconds()` conversion
- [x] `snapshot.json` writes `durationMs` as actual milliseconds (e.g. `1200` for 1.2s, not `1200000000`)
- [x] `checkrun.go` `afterSnap` construction updated
- [x] `snapshotrun.go` `snap.Tests` construction updated
- [x] `snapshotrun.go` `fmtDuration` updated to use `time.Duration` from `DurationMs` reconstruction
- [x] Existing tests that create `TestSummary` updated
- [x] `go test ./...` passes
- [x] `go build ./...` succeeds

---

## Fix 4: Tighten JWT regex (FP-1)

**Problem:** The JWT regex `^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$` matches version strings like `"1.0.0"`.

**Root cause:** `internal/engine/normalizer.go:38` — regex is too broad.

**Fix:** Require JWTs to start with `eyJ` (the base64url-encoded `{"` prefix that all JWT headers begin with).

**Acceptance criteria:**
- [x] JWT regex changed to `^eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`
- [x] `"1.0.0"` normalizes to `"string"`, not `"token"`
- [x] `"v2.1.3"` normalizes to `"string"`, not `"token"`
- [x] Real JWT (starting with `eyJ`) still normalizes to `"token"`
- [x] Existing test `TestNormalize_primitives` updated with version string cases
- [x] New test: `TestNormalize_semverNotToken` — verifies version strings are not classified as tokens
- [x] New test: `TestNormalize_realJwtIsToken` — verifies real JWTs still match
- [x] `go test ./internal/engine/...` passes

---

## Fix 5: Expand dynamic keys list (FP-2)

**Problem:** The `dynamicKeys` map is missing common request-scoped fields like `requestId`, `traceId`, `correlationId`.

**Root cause:** `internal/engine/normalizer.go:16-33` — incomplete list.

**Fix:** Add 12 more common dynamic field names.

**Acceptance criteria:**
- [x] Added: `requestId`, `request_id`, `traceId`, `trace_id`, `correlationId`, `correlation_id`, `spanId`, `span_id`, `parentId`, `parent_id`, `buildId`, `build_id`
- [x] Existing `TestNormalize_dynamicKeysStripped` updated to test new keys
- [x] New test: `TestNormalize_requestScopedFieldsStripped` — verifies all new keys are stripped
- [x] `go test ./internal/engine/...` passes

---

## Fix 6: Fix test regression display (FP-3)

**Problem:** Test regression shows "4 → 4" (passed counts) instead of "0 → 1" (failed counts).

**Root cause:** `internal/engine/diff.go:68-74` — `Before` and `After` use `Tests.Passed` instead of `Tests.Failed`.

**Fix:** Show failed counts in `Before`/`After` and update the message.

**Acceptance criteria:**
- [x] `diff.go` test finding uses `before.Tests.Failed` and `after.Tests.Failed`
- [x] Message updated: "Tests: 0 -> 1 failing (1 new failure(s))"
- [x] JSON `before`/`after` fields for test findings show failed counts
- [x] Existing test `TestDiffSnapshots_testRegression_critical` updated
- [x] `go test ./internal/engine/...` passes

---

## Fix 7: Fix Go syntax leak in schema diff display (ISSUE-1)

**Problem:** `FormatFieldChanges` uses `fmt.Sprintf("%v", ...)` for complex types, producing Go syntax like `map[email:string]`.

**Root cause:** `internal/engine/schemadiff.go:106,112` — `fmt.Sprintf("%v", ...)` on `any` values.

**Fix:** Use `json.Marshal` for complex types, `fmt.Sprintf` for simple strings.

**Acceptance criteria:**
- [x] `FormatFieldChanges` uses a `formatSchemaValue` helper that JSON-encodes complex types
- [x] Array/object values render as JSON (e.g. `[{"email":"string"}]` not `[map[...]]`)
- [x] Simple string values (e.g. `"string"`, `"number"`) render without quotes
- [x] Existing `TestFormatFieldChanges_output` updated if needed
- [x] New test: `TestFormatFieldChanges_complexTypes` — verifies array/object values render as JSON
- [x] `go test ./internal/engine/...` passes

---

## Verification

After all fixes:
- [x] `go build ./...` succeeds
- [x] `go test ./...` passes (all 12 test packages)
- [x] `go vet ./...` clean
- [x] Manual E2E: `rg snapshot` + `rg check` on fixture with regressions
- [x] Manual E2E: git hook uses absolute binary path (not bypassed by ripgrep)
- [x] Manual E2E: field addition = WARNING (exit 0), field removal = CRITICAL (exit 1)
- [x] Manual E2E: `snapshot.json` has correct `durationMs` in milliseconds (1077, not 1077000000)
- [x] Manual E2E: `"1.0.0"` normalizes as `"string"` not `"token"`
