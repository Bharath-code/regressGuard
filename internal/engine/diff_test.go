package engine

import (
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
		key: {Method: "GET", Path: "/api/user", Status: 200, SchemaHash: "hash-before-12345678", MS: 40},
	})
	after := makeSnap(5, 0, map[string]snapshot.RouteRecord{
		key: {Method: "GET", Path: "/api/user", Status: 200, SchemaHash: "hash-after-87654321", MS: 40},
	})

	result := DiffSnapshots(before, after)

	if !result.HasCritical {
		t.Error("expected HasCritical=true for schema change")
	}
	found := false
	for _, r := range result.Results {
		if r.Type == TypeSchema && r.Route == key {
			found = true
		}
	}
	if !found {
		t.Error("expected a schema-type finding")
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
