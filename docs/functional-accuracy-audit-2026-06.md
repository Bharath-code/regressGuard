# RegressGuard — Functional Accuracy Audit

**Date:** 2026-06-19
**Method:** End-to-end testing against `fixtures/nextjs-app` with live Next.js dev server
**Build:** `go build ./cmd/rg` — all tests pass, binary functional

---

## Executive Summary

The core detection engine works: status changes, schema changes, test failures, and timing regressions are all detected. The JSON contract is valid and parseable. The CLI commands (init, snapshot, check, doctor, status, explain, config, hook) all function.

**However, there are 3 critical bugs and 3 high-impact false-positive sources that undermine the product's core promise.** The most severe: the `rg` binary name conflicts with ripgrep, meaning **git hooks silently run ripgrep instead of RegressGuard — the safety net is bypassed without any warning.**

| Severity | Count | Summary |
|---|---|---|
| **CRITICAL** | 3 | rg/ripgrep name conflict; field-additions are CRITICAL not WARNING; durationMs stores nanoseconds |
| **HIGH** | 3 | JWT regex overmatches; dynamic keys list incomplete; test regression display misleading |
| **MEDIUM** | 2 | Go syntax leaks in schema diff display; route removal from config always CRITICAL |
| **LOW** | 2 | No content-type validation; timing non-determinism |

---

## What Works Correctly (Verified)

| Feature | PRD Reference | Status | Evidence |
|---|---|---|---|
| `rg init` | Feature 1 | PASS | Detects Next.js, test command, server URL, writes config.json |
| `rg snapshot` | Feature 2 | PASS | Runs tests (4 passed), hits 5 routes, hashes schemas, saves snapshot.json |
| `rg check` — clean | Flow E | PASS | "OK No regressions detected", exit 0, "Safe to commit." |
| `rg check` — status regression | Flow F | PASS | 401→500 detected, CRITICAL, exit 1, "Commit blocked." |
| `rg check` — schema regression | Flow F | PASS | `subscription` field removed, field-level diff shown, exit 1 |
| `rg check` — test regression | E4-T3 | PASS | New failing test detected, exit 1 (but display is misleading — see BUG-7) |
| `rg check` — timing regression | Flow G | PASS | 7ms→521ms = WARNING, exit 0, "Commit allowed." |
| `rg check --json` | Flow H | PASS | Valid JSON on stdout, schema matches `docs/json-contract.md` |
| `rg check --since HEAD~1` | E13-T1 | PASS | Scopes to changed routes only, passes through unchanged routes |
| `rg doctor` | Feature 5 | PASS | Checks config, test command, dev server, routes, snapshot, git |
| `rg status` | E12-T1 | PASS | Shows snapshot age, route count, hook status in <200ms |
| `rg explain` | E13-T6 | PASS | Shows before/after comparison for a specific route |
| `rg config get` | Feature 5 | PASS | Reads config values correctly |
| `rg hook install` | Feature 4 | PASS | Creates executable pre-commit hook |
| `rg mcp serve` | E13-T5 | PASS | MCP server starts on stdio transport |
| Schema normalization | Module 3 | PASS | Strips dynamic keys, detects dates/UUIDs, hashes shapes |
| Field-level schema diff | E9-T1 | PASS | Shows `+ field (type, added)` and `- field (type, removed)` |
| Git context | E9-T3 | PASS | Shows changed files since snapshot commit |
| Streak counter | E12-T4 | PASS | "7 clean checks in a row" on pass screen |
| Transient error handling | P0-1 | PASS | Unverified routes → WARNING, not CRITICAL |

---

## Critical Bugs

### BUG-1: `rg` binary name conflicts with ripgrep — SAFETY NET BYPASSED

**Severity:** CRITICAL — the entire product promise is broken
**PRD violation:** Feature 4 AC2 ("Critical regressions block the commit")

**What happens:**
The git hook script contains `RG_HOOK=1 rg check`. On any machine with ripgrep installed (the `rg` command), this runs **ripgrep searching for the string "check" in files** instead of RegressGuard. Ripgrep exits 0, the commit proceeds, and the regression is not detected.

**Proof:**
```
$ which rg
/opt/homebrew/bin/rg

$ rg --version
ripgrep 15.1.0

$ cat .git/hooks/pre-commit
#!/bin/sh
RG_HOOK=1 rg check    # ← runs ripgrep, not RegressGuard
RG_EXIT=$?
if [ $RG_EXIT -eq 1 ]; then
  exit 1
fi

# Result: commit succeeds even with a 503 regression
$ git commit -m "update health route"
[main 26a1f93] update health route    # ← COMMIT SUCCEEDED
```

**Impact:** Every user with ripgrep installed (the majority of developers) has a silently broken safety net. They believe they're protected. They are not.

**Fix options:**
1. **Rename binary to `regressguard`** (safest, loses short name)
2. **Install as `regressguard`, alias `rg` only if no conflict** — install script checks `which rg` and skips alias if ripgrep is found
3. **Hook script uses full path** — `RG_HOOK=1 /usr/local/bin/regressguard check` or `RG_HOOK=1 regressguard check`
4. **Detect conflict on `rg hook install`** — warn the user if `rg` resolves to ripgrep

**Recommended:** Rename to `regressguard` for the hook script and MCP registration. Keep `rg` as an optional convenience alias. The hook script should use the absolute path or the full binary name.

---

### BUG-2: Field additions treated as CRITICAL instead of WARNING

**Severity:** CRITICAL — blocks legitimate backward-compatible changes
**PRD violation:** Section 8.2 Module 5: "WARNING: optional field added/removed"

**What happens:**
When a new field is added to an API response (backward-compatible change), `diff.go:130` treats ALL schema hash mismatches as CRITICAL. The `DiffSchemaShapes` function correctly identifies the change as "added", but the severity is not adjusted — it's always CRITICAL.

**Proof:**
```
# Add newFeature field to /api/profile response
$ rg check
X 1 regression detected
  GET /api/profile    fde32a77  e0ab225e  schema
    + newFeature (string, added)
X Commit blocked.    # ← should be WARNING, exit 0
```

**Code location:** `internal/engine/diff.go:130-142`
```go
if snap.SchemaHash != curr.SchemaHash && snap.SchemaHash != "" && curr.SchemaHash != "" {
    fieldChanges := DiffSchemaShapes(snap.NormalizedSchema, curr.NormalizedSchema)
    results = append(results, CheckResult{
        Severity: SeverityCritical,  // ← ALWAYS critical, regardless of field change type
        ...
    })
}
```

**Impact:** Every time a developer adds a new field to an API response (a normal, backward-compatible change), RegressGuard blocks the commit. This is the most common type of API change, and blocking it makes the tool feel broken.

**Fix:**
```go
fieldChanges := DiffSchemaShapes(snap.NormalizedSchema, curr.NormalizedSchema)
severity := classifySchemaChange(fieldChanges) // added → WARNING, removed/changed → CRITICAL
results = append(results, CheckResult{
    Severity: severity,
    ...
})
```

Add a `classifySchemaChange` function:
- If ALL changes are "added" → `WARNING`
- If ANY change is "removed" or "changed" → `CRITICAL`
- Add config option: `"schemaAdditions": "critical"` for users who want stricter behavior

---

### BUG-3: `durationMs` field stores nanoseconds, not milliseconds

**Severity:** CRITICAL — JSON contract is semantically wrong
**PRD violation:** Section 6.3 ("stdout must contain valid JSON and nothing else") + `docs/json-contract.md`

**What happens:**
`snapshot.TestSummary.Duration` is `time.Duration` (Go int64 nanoseconds) with JSON tag `durationMs`. Go serializes `time.Duration` as the raw nanosecond integer. The field says "milliseconds" but stores nanoseconds.

**Proof:**
```json
{
  "tests": {
    "passed": 4,
    "failed": 0,
    "durationMs": 1190019083    // ← 1.19 seconds in nanoseconds
  }
}
```
- As nanoseconds: 1.19 seconds (correct actual duration)
- As milliseconds (what the field name says): 1,190,019 ms = ~20 minutes (absurd)

**Code location:** `internal/snapshot/snapshot.go:35`
```go
type TestSummary struct {
    Duration time.Duration `json:"durationMs"`  // ← time.Duration serializes as nanoseconds
}
```

**Impact:** Any consumer of the JSON contract (MCP tools, future hosted ingest, `jq` pipelines) will misinterpret test duration by 1,000,000x. The hosted layer's history retention would store garbage values.

**Fix:**
```go
type TestSummary struct {
    DurationMs int64 `json:"durationMs"`
}
// Set: DurationMs: duration.Milliseconds()
```

---

## High-Impact False Positive Sources

### FP-1: JWT regex overmatches semver and dotted strings

**Severity:** HIGH — causes both false positives and false negatives
**Location:** `internal/engine/normalizer.go:38`

**What happens:**
The JWT regex `^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$` matches any string with two dots where each segment is alphanumeric. This includes:
- `"1.0.0"` → normalized as `"token"` (it's a version string)
- `"v2.1.3"` → normalized as `"token"`
- `"2026.06.19"` → normalized as `"token"`
- `"a.b.c"` → normalized as `"token"`

**Proof:**
```json
// Health endpoint returns {"status": "ok", "version": "1.0.0"}
// Snapshot stores:
"normalizedSchema": {
    "status": "string",
    "version": "token"    // ← "1.0.0" was classified as a JWT
}
```

**Impact:**
- **False negative:** If `version` changes from `"1.0.0"` to `"2.0.0"`, both normalize to `"token"` — no schema change detected. (Arguably correct — version bumps shouldn't be schema changes.)
- **False positive:** If `version` changes from `"1.0.0"` to `"active"`, the hash changes (`"token"` → `"string"`) — flagged as a schema change when only the value changed.

**Fix:** Tighten the JWT regex to require characteristics of actual JWTs:
```go
// Option A: require eyJ prefix (JWT headers start with eyJ in base64)
reJWT = regexp.MustCompile(`^eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)

// Option B: require minimum segment length (JWT segments are at least 16 chars)
reJWT = regexp.MustCompile(`^[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}$`)

// Option C: check for typical JWT structure (header.payload.signature with base64url)
// and verify the first segment decodes to a JSON object with "alg" and "typ"
```

---

### FP-2: Dynamic keys list missing common request-scoped fields

**Severity:** HIGH — causes false CRITICAL on first run with instrumented APIs
**Location:** `internal/engine/normalizer.go:16-33`

**What happens:**
The `dynamicKeys` map strips fields whose values change every run. But it's missing extremely common fields in instrumented API responses:

| Missing key | Frequency | Why it's dynamic |
|---|---|---|
| `requestId` / `request_id` | Very common | Unique per request (traceability) |
| `traceId` / `trace_id` | Very common | Unique per request (distributed tracing) |
| `correlationId` / `correlation_id` | Common | Unique per request flow |
| `spanId` / `span_id` | Common (OpenTelemetry) | Unique per span |
| `xRequestId` / `x_request_id` | Common | HTTP header echoed in response |
| `buildId` / `build_id` | Common (CI/CD) | Changes per deployment |
| `deployId` / `deploy_id` | Common (CI/CD) | Changes per deployment |

**Proof:**
```
# Add requestId and traceId to health route
$ rg check
X 1 regression detected
  GET /api/health    053a9e75  d042d62b  schema
    + requestId (string, added)
    + traceId (string, added)
X Commit blocked.    # ← false positive: these fields change every request
```

**Impact:** Any API with request tracing (which is most production APIs) will produce false CRITICAL findings on every run, because the request-scoped fields have different values each time. The user must manually add each field to `ignoreFields` — high friction.

**Fix:** Expand the `dynamicKeys` map:
```go
var dynamicKeys = map[string]bool{
    // Existing...
    "createdAt": true, "updatedAt": true, "timestamp": true, ...
    // Add request-scoped fields:
    "requestId": true, "request_id": true,
    "traceId": true, "trace_id": true,
    "correlationId": true, "correlation_id": true,
    "spanId": true, "span_id": true,
    "parentId": true, "parent_id": true,
    "xRequestId": true, "x_request_id": true,
    // Add deployment-scoped fields:
    "buildId": true, "build_id": true,
    "deployId": true, "deploy_id": true,
    "revision": true, "commit": true,
}
```

Also add pattern-based auto-detection: any key ending in `Id` or `_id` that contains a UUID-like value could be auto-stripped (configurable).

---

### FP-3: Test regression display shows passed counts, not failed counts

**Severity:** HIGH (UX) — misleading output undermines trust
**Location:** `internal/engine/diff.go:68-74`

**What happens:**
When a new test fails, the diff engine stores `before.Tests.Passed` and `after.Tests.Passed` in the `Before` and `After` fields. But the regression is in the failed count, not the passed count. The human table shows "4 → 4" which looks like nothing changed.

**Proof:**
```
# Snapshot: 4 passed, 0 failed
# After: 4 passed, 1 failed (new broken test)

$ rg check
X 3 regressions detected
                                        4         4         tests    # ← "4 → 4" is misleading
  GET /api/auth/verify                  401       500       status
  GET /api/profile                      fde32a77  caedbbfe  schema
```

The JSON is slightly better but still confusing:
```json
{
    "type": "tests",
    "before": 4,
    "after": 4,
    "message": "Tests: 4 passed -> 4 passed (1 new failure(s))"
}
```

**Fix:** Show failed counts, not passed counts:
```go
// In diff.go:
results = append(results, CheckResult{
    Severity: SeverityCritical,
    Type:     TypeTests,
    Before:   before.Tests.Failed,  // ← was before.Tests.Passed
    After:    after.Tests.Failed,   // ← was after.Tests.Passed
    Message: fmt.Sprintf(
        "Tests: %d -> %d failing (%d new failure(s))",
        before.Tests.Failed, after.Tests.Failed, delta,
    ),
})
```

---

## Medium-Impact Issues

### ISSUE-1: Go syntax leaks in schema diff display

**Severity:** MEDIUM (UX)
**Location:** `internal/engine/schemadiff.go:106,112` (`FormatFieldChanges`)

**What happens:**
When a field's normalized value is a complex type (array/object), `FormatFieldChanges` uses `fmt.Sprintf("%v", ...)` which produces Go syntax like `map[email:string name:string]` instead of JSON.

**Proof:**
```
# When users array changes from [{...}] to []
~ users ([map[email:string name:string role:string subscription:string]] -> empty_array)
```

**Fix:** Use `json.Marshal` for complex types:
```go
func formatValue(v any) string {
    b, err := json.Marshal(v)
    if err != nil {
        return fmt.Sprintf("%v", v)
    }
    return string(b)
}
```

---

### ISSUE-2: Route removed from config is always CRITICAL

**Severity:** MEDIUM
**Location:** `internal/engine/diff.go:81-91`
**Staff review reference:** P0-2 (deferred)

**What happens:**
When a user intentionally removes a route from `config.json`, `rg check` flags it as CRITICAL "route no longer present in current run." This blocks the commit for an intentional config change.

**Current behavior (per staff review):** Deferred — "removing a route from config arguably *should* still warn."

**Recommendation:** Downgrade to WARNING with a clear message: "Route GET /api/old was removed from config. If intentional, run rg snapshot." This matches the pattern for other intentional changes.

---

## Low-Impact Issues

### ISSUE-3: No content-type validation

**Severity:** LOW
**Location:** `internal/engine/normalizer.go:137-143` (`NormalizeAndHash`)

If a route returns HTML (e.g., a 404 page), `json.Unmarshal` fails and `NormalizeAndHash` returns an empty string. Two runs both get empty strings (no false positive), but a switch from JSON to HTML would be flagged as a schema change. This is arguably correct behavior.

### ISSUE-4: Timing non-determinism

**Severity:** LOW
**Location:** `internal/engine/diff.go:145-157`

Wall-clock timing varies between runs due to system load, GC pauses, etc. The threshold (>200ms AND >50% increase) is reasonable and rarely fires on fast machines. On slow CI machines or under load, it may produce occasional false warnings. This is an inherent challenge with timing-based detection.

---

## How to Reduce False Positives — Prioritized Fix List

| Priority | Fix | Effort | FP Reduction | Status |
|---|---|---|---|---|
| **P0** | **Fix `rg` name conflict** — rename to `regressguard` for hooks/MCP | Medium | N/A (fixes safety) | **NOT STARTED** |
| **P0** | **Field additions → WARNING, not CRITICAL** | Small | Eliminates #1 FP source | **NOT STARTED** |
| **P0** | **Fix `durationMs` serialization** | Trivial | Fixes JSON contract | **NOT STARTED** |
| **P1** | **Expand dynamic keys list** (+12 keys) | Trivial | Eliminates #2 FP source | **NOT STARTED** |
| **P1** | **Tighten JWT regex** (require `eyJ` prefix) | Trivial | Eliminates #3 FP source | **NOT STARTED** |
| **P1** | **Fix test regression display** (show failed, not passed) | Trivial | Fixes UX trust | **NOT STARTED** |
| **P2** | **Fix Go syntax leak in schema diff** | Trivial | Fixes UX polish | **NOT STARTED** |
| **P2** | **Route removal → WARNING, not CRITICAL** | Small | Eliminates intentional-change FP | **NOT STARTED** |
| **P2** | **Add `Id`/`_id` suffix auto-detection** (configurable) | Medium | Prevents future FP from new fields | **NOT STARTED** |

---

## Additional Accuracy Recommendations

### 1. Add a "snapshot stability check" on `rg snapshot`
After capturing a snapshot, immediately hit the routes a second time and compare schema hashes. If any route's hash changes between two consecutive hits, warn the user: "Route GET /api/users has an unstable schema — likely contains dynamic fields not in the ignore list. Add them to ignoreFields or the route will produce false positives."

This catches the `requestId`/`traceId` problem at snapshot time, before the user ever runs `rg check`.

### 2. Add a `--strict` flag for schema additions
Some teams want field additions to be CRITICAL (they need API contract review for any change). Add `rg check --strict` or `config.json: "schemaAdditions": "critical"` to let users choose.

### 3. Add array element sampling
Instead of only checking the first element of an array, check the first 3 elements. If they have different shapes, store the union of shapes. This catches heterogeneous arrays without adding significant overhead.

### 4. Add a "dry run" mode for `rg check`
`rg check --dry-run` would hit routes and compare schemas but always exit 0. This lets users see what would be flagged without blocking commits. Useful for initial setup and tuning `ignoreFields`.

### 5. Add `rg tune` command
An interactive command that hits each route twice, identifies fields with unstable values, and suggests additions to `ignoreFields`. This eliminates the manual trial-and-error of finding dynamic fields.

### 6. Snapshot versioning and migration
Add a `migrate` function that can upgrade old snapshots to new formats. When the normalizer logic changes (e.g., JWT regex tightening), old snapshots will have different hashes. The migration function would re-normalize old snapshots using the new logic.

---

## Test Coverage Gaps

| Package | Tests | Risk | Recommendation |
|---|---|---|---|
| `snapshot/integrity.go` | None | HMAC is a security primitive | Add tests for HMAC write/verify/tamper detection |
| `statusrun` | None | User-facing command | Add tests for all status output paths |
| `doctorrun` | None | User-facing command | Add tests for all doctor checks |
| `watchrun` | None | User-facing command | Add tests for debounce, file filtering |
| `upgraderun` | None | Self-update is security-sensitive | Add tests for checksum verification, binary replacement |
| `state` | None | Streak/celebration logic | Add tests for increment/reset/first-run |
| `configrun` | None | User-facing command | Add tests for get/set operations |

**E2E test gap:** No test runs the full `init → snapshot → AI edit → check → block commit` loop against the fixture project. This is the most important test to add — it would have caught BUG-1 (the ripgrep conflict).
