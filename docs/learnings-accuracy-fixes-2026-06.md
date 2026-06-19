# Functional Accuracy Fixes — Learnings

**Date:** 2026-06-19
**Feature:** Accuracy and false-positive reduction for RegressGuard v1
**PRD sections:** §8.2 (severity rules), §9 (accuracy strategy), §6.3 (JSON contract)

---

## Why This Was Needed

RegressGuard's entire product promise is: "Before you commit, know what broke." If the tool produces false positives (blocks commits for non-breaking changes) or false negatives (misses real regressions), developers uninstall. Accuracy is the product — not features.

The functional accuracy audit (`docs/functional-accuracy-audit-2026-06.md`) found 3 critical bugs and 4 false-positive sources that undermined this promise. The most severe: the `rg` binary name conflicted with ripgrep, meaning **git hooks silently ran ripgrep instead of RegressGuard** — the safety net was completely bypassed without any warning.

---

## What Was Fixed

### 1. Git hook binary resolution (BUG-1) — the safety net was broken

**The problem:** The hook script ran `RG_HOOK=1 rg check`. On machines with ripgrep installed (the majority of developers), `rg` resolves to ripgrep. Ripgrep searches for the string "check" in files, exits 0, and the commit proceeds — even with a 503 regression.

**The fix:** `hookrun.go` now resolves the absolute binary path at install time via `os.Executable()`. The hook script uses `RG_HOOK=1 /absolute/path/to/rg check` instead of bare `rg check`. If the path can't be resolved, it falls back to `regressguard`. A ripgrep conflict warning is printed on install.

**Why it matters:** This was a silent safety failure. Users believed they were protected. They were not. Every commit with a regression was going through.

**How it's used:** `rg hook install` automatically resolves the binary path and injects it into the hook script. No user action needed.

### 2. Schema severity classification (BUG-2) — field additions blocked commits

**The problem:** All schema hash mismatches were CRITICAL. Adding a new field to an API response (backward-compatible) blocked the commit. This is the most common type of API change.

**The fix:** `classifySchemaChange()` in `diff.go` examines the field-level diff. If all changes are "added" → WARNING (exit 0). If any change is "removed" or "changed" → CRITICAL (exit 1). This matches PRD §8.2: "WARNING: optional field added/removed."

**Why it matters:** False CRITICAL findings on backward-compatible changes make the tool feel broken. Developers disable the hook or stop running `rg check`. This was the #1 false-positive source for real-world usage.

**How it's used:** When an AI agent adds a new field to a response, `rg check` now reports it as a non-blocking WARNING ("Commit allowed.") instead of blocking the commit. When a field is removed (breaking change), it still blocks.

### 3. Duration serialization (BUG-3) — JSON contract was semantically wrong

**The problem:** `TestSummary.Duration` was `time.Duration` (Go int64 nanoseconds) with JSON tag `durationMs`. The field said "milliseconds" but stored nanoseconds. A 1.2s test run was serialized as `1077000000` instead of `1077`.

**The fix:** Changed to `DurationMs int64` with `.Milliseconds()` conversion at all call sites.

**Why it matters:** The JSON contract is the integration surface for MCP tools and the future hosted layer. Any consumer would misinterpret test duration by 1,000,000x.

### 4. JWT regex tightening (FP-1) — version strings matched as tokens

**The problem:** The JWT regex `^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$` matched any string with two dots. `"1.0.0"`, `"v2.1.3"`, `"2026.06.19"` were all normalized as `"token"`.

**The fix:** Regex now requires `eyJ` prefix (the base64url-encoded `{"` that all JWT headers start with).

**Why it matters:** Version strings in API responses (e.g., `{"version": "1.0.0"}`) were being normalized as tokens. If the version changed from `"1.0.0"` to `"active"`, the schema hash changed — false positive. If it changed from `"1.0.0"` to `"2.0.0"`, both were `"token"` — false negative.

### 5. Dynamic keys expansion (FP-2) — request-scoped fields caused false CRITICAL

**The problem:** `requestId`, `traceId`, `correlationId`, `spanId`, `buildId` and their snake_case variants were not in the `dynamicKeys` map. Any instrumented API (most production APIs) would get false CRITICAL findings on every run because these fields have different values each request.

**The fix:** Added 12 more common dynamic field names to the `dynamicKeys` map.

**Why it matters:** This was the #2 false-positive source. Without these keys, the tool was unusable on any API with request tracing (OpenTelemetry, DataDog, etc.).

### 6. Test regression display (FP-3) — misleading "4 → 4" output

**The problem:** Test regression displayed passed counts ("4 → 4") instead of failed counts ("0 → 1"). Users saw "4 → 4" and thought nothing changed.

**The fix:** `Before`/`After` fields now use `Tests.Failed` instead of `Tests.Passed`. Message: "Tests: 0 -> 1 failing (1 new failure(s))."

### 7. Schema diff display (ISSUE-1) — Go syntax leaked into output

**The problem:** `FormatFieldChanges` used `fmt.Sprintf("%v", ...)` for complex types, producing Go syntax like `[map[email:string name:string]]` instead of JSON.

**The fix:** `formatSchemaValue()` uses `json.Marshal` for complex types (maps, arrays), plain strings for type tokens.

---

## How These Fixes Solve the Problem

| Fix | Before | After |
|---|---|---|
| BUG-1 | Hook runs ripgrep → commit always succeeds | Hook uses absolute path → regression blocks commit |
| BUG-2 | Field added → CRITICAL, commit blocked | Field added → WARNING, commit allowed |
| BUG-3 | `durationMs: 1077000000` (nanoseconds) | `durationMs: 1077` (milliseconds) |
| FP-1 | `"1.0.0"` → `"token"` (false classification) | `"1.0.0"` → `"string"` (correct) |
| FP-2 | `requestId` in response → false CRITICAL | `requestId` stripped → no false positive |
| FP-3 | "Tests: 4 → 4" (misleading) | "Tests: 0 → 1 failing" (clear) |
| ISSUE-1 | `[map[...]]` (Go syntax in output) | `[{"email":"string"}]` (JSON) |

---

## Key Lessons

1. **Binary name conflicts are silent killers.** `rg` is one of the most popular CLI tools (ripgrep). Using the same name without conflict detection means the safety net is silently bypassed. Always resolve absolute paths for security-critical scripts.

2. **Not all schema changes are breaking.** Adding a field is backward-compatible. Removing or changing a field is breaking. The diff engine must distinguish between these — a blanket "schema changed = CRITICAL" produces too many false positives.

3. **Go's `time.Duration` serializes as nanoseconds.** Using it with a JSON tag like `durationMs` is a semantic mismatch. Always convert to the unit the JSON tag claims.

4. **Regex patterns for security tokens must be specific.** A pattern that matches `X.Y.Z` will match version strings, dates, and dotted identifiers. Require characteristics unique to the target (e.g., `eyJ` prefix for JWTs).

5. **Dynamic field lists must cover real-world instrumentation.** Request tracing is ubiquitous in production APIs. Missing `requestId`/`traceId` makes the tool unusable on any instrumented API.

6. **Display values must match the semantic meaning.** Showing passed counts for a test regression is misleading even if the detection is correct. Users read the display, not the raw data.

7. **E2E testing against real fixtures catches bugs unit tests miss.** The ripgrep conflict was invisible to unit tests (which use `bytes.Buffer`, not real PATH resolution). Only an E2E test that runs the full `init → snapshot → check → commit` loop would have caught it.

---

## Files Changed

| File | Change |
|---|---|
| `internal/hookrun/hookrun.go` | Binary path resolution, ripgrep detection, hook script template |
| `internal/hookrun/hookrun_test.go` | Updated tests, added `TestInstall_hookScriptUsesAbsolutePath`, `TestInstall_warnsOnRipgrepConflict` |
| `internal/engine/diff.go` | `classifySchemaChange()` function, test finding uses failed counts, hash prefix guard |
| `internal/engine/diff_test.go` | Updated `TestDiffSnapshots_testRegression_critical`, added 4 new schema severity tests |
| `internal/engine/normalizer.go` | JWT regex tightened to `eyJ` prefix, 12 new dynamic keys |
| `internal/engine/normalizer_test.go` | Added `TestNormalize_semverNotToken`, `TestNormalize_realJwtIsToken`, `TestNormalize_requestScopedFieldsStripped` |
| `internal/engine/schemadiff.go` | `formatSchemaValue()` uses `json.Marshal` for complex types |
| `internal/engine/schemadiff_test.go` | Added `TestFormatFieldChanges_complexTypes` |
| `internal/snapshot/snapshot.go` | `Duration time.Duration` → `DurationMs int64` |
| `internal/snapshotrun/snapshotrun.go` | Updated call sites for `DurationMs` |
| `internal/checkrun/checkrun.go` | Updated `afterSnap` construction for `DurationMs` |
