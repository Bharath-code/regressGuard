package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/Bharath-code/regressguard/internal/snapshot"
)

// helpers

func makeSnap(passed, failed int, routes map[string]snapshot.RouteRecord) snapshot.Snapshot {
	return snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Tests:     snapshot.TestSummary{Passed: passed, Failed: failed},
		Routes:    routes,
	}
}

func routeRecord(status int, hash string, ms int64) snapshot.RouteRecord {
	return snapshot.RouteRecord{
		Method:     "GET",
		Path:       "/api/test",
		Status:     status,
		SchemaHash: hash,
		MS:         ms,
	}
}

// E4-T3: test diff

func TestDiffSnapshots_testRegression_critical(t *testing.T) {
	before := makeSnap(42, 0, nil)
	after := makeSnap(40, 2, nil)

	result := DiffSnapshots(before, after)

	if !result.HasCritical {
		t.Error("expected HasCritical=true when tests newly fail")
	}
	if result.CriticalCount != 1 {
		t.Errorf("expected 1 critical finding, got %d", result.CriticalCount)
	}
	if result.Results[0].Type != TypeTests {
		t.Errorf("expected type=%q, got %q", TypeTests, result.Results[0].Type)
	}
	if result.Results[0].Before != 0 {
		t.Errorf("expected Before=0 (failed count), got %v", result.Results[0].Before)
	}
	if result.Results[0].After != 2 {
		t.Errorf("expected After=2 (failed count), got %v", result.Results[0].After)
	}
}

func TestDiffSnapshots_testCountImproved_noRegression(t *testing.T) {
	// More tests passing is not a regression.
	before := makeSnap(40, 0, nil)
	after := makeSnap(42, 0, nil)

	result := DiffSnapshots(before, after)

	if result.HasCritical {
		t.Error("expected no critical when test count improves")
	}
}

func TestDiffSnapshots_sameFailCount_noRegression(t *testing.T) {
	// Same failure count — not a new regression.
	before := makeSnap(40, 2, nil)
	after := makeSnap(40, 2, nil)

	result := DiffSnapshots(before, after)

	if result.HasCritical {
		t.Error("expected no critical when failure count unchanged")
	}
}

// E4-T4: status code diff

func TestDiffSnapshots_statusChange_critical(t *testing.T) {
	key := "GET /api/auth/verify"
	before := makeSnap(10, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/auth/verify", Status: 200, SchemaHash: "abc", MS: 50},
	})
	after := makeSnap(10, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/auth/verify", Status: 401, SchemaHash: "abc", MS: 50},
	})

	result := DiffSnapshots(before, after)

	if !result.HasCritical {
		t.Error("expected HasCritical=true for status change")
	}
	found := false
	for _, r := range result.Results {
		if r.Type == TypeStatus && r.Route == key {
			found = true
			if r.Severity != SeverityCritical {
				t.Errorf("expected CRITICAL severity, got %q", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected a status-type finding for the route")
	}
}

func TestDiffSnapshots_statusUnchanged_noRegression(t *testing.T) {
	key := "GET /api/health"
	rec := snapshot.RouteRecord{Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "abc", MS: 30}
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{key: rec})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{key: rec})

	result := DiffSnapshots(before, after)

	if result.HasCritical || result.HasWarning {
		t.Errorf("expected clean pass, got critical=%v warning=%v", result.HasCritical, result.HasWarning)
	}
	if result.PassedRoutes != 1 {
		t.Errorf("expected 1 passed route, got %d", result.PassedRoutes)
	}
}

// E4-T5: schema diff

func TestDiffSnapshots_schemaChange_critical(t *testing.T) {
	key := "GET /api/user"
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/user", Status: 200, SchemaHash: "hash-before-12345678", MS: 40,
			NormalizedSchema: []byte(`{"name":"string","subscription":"string"}`)},
	})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/user", Status: 200, SchemaHash: "hash-after-87654321", MS: 40,
			NormalizedSchema: []byte(`{"name":"string"}`)},
	})

	result := DiffSnapshots(before, after)

	if !result.HasCritical {
		t.Error("expected HasCritical=true for schema field removal")
	}
	found := false
	for _, r := range result.Results {
		if r.Type == TypeSchema && r.Route == key {
			found = true
			if r.Severity != SeverityCritical {
				t.Errorf("expected CRITICAL severity for field removal, got %q", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected a schema-type finding")
	}
}

func TestDiffSnapshots_schemaFieldAdded_warning(t *testing.T) {
	key := "GET /api/profile"
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "hash-before-12345678", MS: 40,
			NormalizedSchema: []byte(`{"name":"string","email":"string"}`)},
	})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "hash-after-87654321", MS: 40,
			NormalizedSchema: []byte(`{"name":"string","email":"string","newFeature":"string"}`)},
	})

	result := DiffSnapshots(before, after)

	if result.HasCritical {
		t.Error("field addition should NOT be CRITICAL")
	}
	if !result.HasWarning {
		t.Error("expected HasWarning=true for field addition")
	}
}

func TestDiffSnapshots_schemaFieldRemoved_critical(t *testing.T) {
	key := "GET /api/profile"
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "hash-before-12345678", MS: 40,
			NormalizedSchema: []byte(`{"name":"string","subscription":"string"}`)},
	})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "hash-after-87654321", MS: 40,
			NormalizedSchema: []byte(`{"name":"string"}`)},
	})

	result := DiffSnapshots(before, after)

	if !result.HasCritical {
		t.Error("field removal should be CRITICAL")
	}
}

func TestDiffSnapshots_schemaMixedChanges_critical(t *testing.T) {
	key := "GET /api/profile"
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "hash-before-12345678", MS: 40,
			NormalizedSchema: []byte(`{"name":"string","subscription":"string"}`)},
	})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "hash-after-87654321", MS: 40,
			NormalizedSchema: []byte(`{"name":"string","tier":"string"}`)},
	})

	result := DiffSnapshots(before, after)

	if !result.HasCritical {
		t.Error("mixed changes with any removal should be CRITICAL")
	}
}

func TestDiffSnapshots_schemaAddedRoute_notCountedAsPassed(t *testing.T) {
	warned := "GET /api/profile"
	clean := "GET /api/health"
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		warned: {Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "aaaahash-before123", MS: 40,
			NormalizedSchema: []byte(`{"name":"string"}`)},
		clean:  {Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "bbbbhash-stable456", MS: 30},
	})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		warned: {Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "cccchash-after789", MS: 40,
			NormalizedSchema: []byte(`{"name":"string","newField":"string"}`)},
		clean: {Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "bbbbhash-stable456", MS: 30},
	})

	result := DiffSnapshots(before, after)

	if result.PassedRoutes != 1 {
		t.Errorf("warning-only schema route must not count as unchanged; expected 1 passed, got %d", result.PassedRoutes)
	}
}

func TestDiffSnapshots_schemaUnchanged_noRegression(t *testing.T) {
	key := "GET /api/profile"
	rec := snapshot.RouteRecord{Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "stable-hash-12345678", MS: 60}
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{key: rec})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{key: rec})

	result := DiffSnapshots(before, after)

	if result.HasCritical {
		t.Error("expected no critical when schema unchanged")
	}
}

// E4-T6: timing diff

func TestDiffSnapshots_timingRegression_warning(t *testing.T) {
	key := "GET /api/profile"
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "abc", MS: 40},
	})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		// +420ms, which is >200ms AND >50% of 40ms baseline.
		key: {Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "abc", MS: 460},
	})

	result := DiffSnapshots(before, after)

	if result.HasCritical {
		t.Error("timing regression should not be CRITICAL")
	}
	if !result.HasWarning {
		t.Error("expected HasWarning=true for large timing regression")
	}
	found := false
	for _, r := range result.Results {
		if r.Type == TypeTiming && r.Route == key {
			found = true
			if r.Severity != SeverityWarning {
				t.Errorf("expected WARNING severity, got %q", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected a timing-type finding")
	}
}

// P1-3: a route whose only change is a non-blocking timing WARNING is reported
// separately and must NOT be tallied in "Routes: N unchanged."
func TestDiffSnapshots_timingWarningRoute_notCountedAsPassed(t *testing.T) {
	warned := "GET /api/profile"
	clean := "GET /api/health"
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		warned: {Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "abc", MS: 40},
		clean:  {Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "def", MS: 30},
	})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		// +420ms — a timing WARNING.
		warned: {Method: "GET", Path: "/api/profile", Status: 200, SchemaHash: "abc", MS: 460},
		// unchanged.
		clean: {Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "def", MS: 30},
	})

	result := DiffSnapshots(before, after)

	if !result.HasWarning {
		t.Fatal("expected a timing WARNING")
	}
	if result.PassedRoutes != 1 {
		t.Errorf("a warning-only route must not count as unchanged; expected 1 passed (the clean route), got %d", result.PassedRoutes)
	}
}

func TestDiffSnapshots_timingSmallDelta_noWarning(t *testing.T) {
	// +50ms — below the 200ms threshold.
	key := "GET /api/health"
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "abc", MS: 30},
	})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "abc", MS: 80},
	})

	result := DiffSnapshots(before, after)

	if result.HasWarning {
		t.Error("expected no warning for small timing delta")
	}
}

func TestDiffSnapshots_timingLargeDeltaSmallPercent_noWarning(t *testing.T) {
	// +250ms but only 5% increase from a 5000ms baseline — below 50% threshold.
	key := "GET /api/slow"
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/slow", Status: 200, SchemaHash: "abc", MS: 5000},
	})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/slow", Status: 200, SchemaHash: "abc", MS: 5250},
	})

	result := DiffSnapshots(before, after)

	if result.HasWarning {
		t.Error("expected no warning when percentage increase is below threshold")
	}
}

func TestDiffSnapshots_routeDisappeared_critical(t *testing.T) {
	key := "GET /api/users"
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/users", Status: 200, SchemaHash: "abc", MS: 30},
	})
	// Route not present in after.
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{})

	result := DiffSnapshots(before, after)

	if !result.HasCritical {
		t.Error("expected CRITICAL when a route disappears")
	}
}

// P0-1: a route that could not be measured during the check (transient
// timeout / connection error) must be a non-blocking WARNING, never CRITICAL.
func TestDiffSnapshots_unverifiedRoute_warningNotCritical(t *testing.T) {
	key := "GET /api/users"
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/users", Status: 200, SchemaHash: "abc", MS: 30},
	})
	// Route present in after but marked unverified (e.g. it timed out mid-check).
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/users", Unverified: true, UnverifiedReason: "request failed: timeout"},
	})

	result := DiffSnapshots(before, after)

	if result.HasCritical {
		t.Errorf("a transient/unverified route must not be CRITICAL\nresults: %+v", result.Results)
	}
	if !result.HasWarning {
		t.Error("an unverified route should surface as a WARNING")
	}
	found := false
	for _, r := range result.Results {
		if r.Type == TypeUnverified && r.Route == key {
			found = true
			if r.Severity != SeverityWarning {
				t.Errorf("expected WARNING severity, got %q", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected an unverified-type finding for the route")
	}
}

func TestDiffSnapshots_unverifiedRoute_notCountedAsPassed(t *testing.T) {
	key := "GET /api/users"
	before := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/users", Status: 200, SchemaHash: "abc", MS: 30},
	})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/users", Unverified: true, UnverifiedReason: "timeout"},
	})

	result := DiffSnapshots(before, after)

	if result.PassedRoutes != 0 {
		t.Errorf("an unverified route must not be counted as passed, got PassedRoutes=%d", result.PassedRoutes)
	}
}

func TestDiffSnapshots_cleanPass_zeroCounts(t *testing.T) {
	key := "GET /api/health"
	rec := snapshot.RouteRecord{Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "abc", MS: 20}
	before := makeSnap(10, 0, map[string]snapshot.RouteRecord{key: rec})
	after := makeSnap(10, 0, map[string]snapshot.RouteRecord{key: rec})

	result := DiffSnapshots(before, after)

	if result.CriticalCount != 0 || result.WarningCount != 0 {
		t.Errorf("expected 0/0 counts, got critical=%d warning=%d", result.CriticalCount, result.WarningCount)
	}
	if result.PassedRoutes != 1 {
		t.Errorf("expected 1 passed route, got %d", result.PassedRoutes)
	}
}

func TestDiffSnapshots_newNamedFailure_sameCount_critical(t *testing.T) {
	before := makeSnap(41, 1, nil)
	before.Tests.FailedNames = []string{"old flaky test"}
	after := makeSnap(41, 1, nil)
	after.Tests.FailedNames = []string{"rejects bad password"}

	result := DiffSnapshots(before, after)
	if !result.HasCritical {
		t.Fatal("expected CRITICAL for new named failure with unchanged count")
	}
	found := false
	for _, r := range result.Results {
		if r.Type == TypeTests && r.Severity == SeverityCritical {
			found = true
			if !strings.Contains(r.Message, "rejects bad password") {
				t.Errorf("finding should name the newly failing test, got %q", r.Message)
			}
		}
	}
	if !found {
		t.Error("expected a tests-type CRITICAL finding")
	}
}

func TestDiffSnapshots_sameNamedFailures_noFinding(t *testing.T) {
	before := makeSnap(41, 1, nil)
	before.Tests.FailedNames = []string{"known failure"}
	after := makeSnap(41, 1, nil)
	after.Tests.FailedNames = []string{"known failure"}

	result := DiffSnapshots(before, after)
	for _, r := range result.Results {
		if r.Type == TypeTests {
			t.Errorf("expected no tests finding, got %+v", r)
		}
	}
}

func TestDiffSnapshots_oldSnapshotNoNames_fallsBackToCount(t *testing.T) {
	// Baseline from an old rg version: failures recorded but no names.
	before := makeSnap(41, 1, nil)
	after := makeSnap(41, 1, nil)
	after.Tests.FailedNames = []string{"rejects bad password"}

	result := DiffSnapshots(before, after)
	for _, r := range result.Results {
		if r.Type == TypeTests {
			t.Errorf("expected count-based fallback (no finding), got %+v", r)
		}
	}
}

func TestDiffSnapshots_countIncrease_namesUnparsed_stillCritical(t *testing.T) {
	before := makeSnap(42, 0, nil)
	after := makeSnap(41, 1, nil) // no names parsed

	result := DiffSnapshots(before, after)
	if !result.HasCritical {
		t.Fatal("count increase without names must remain CRITICAL")
	}
}
