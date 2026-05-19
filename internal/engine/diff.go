// Package engine — diff.go computes the delta between two snapshots.
// Severity rules follow PRD Section 8.2 Module 5 and Section 10.3.
package engine

import (
	"fmt"

	"github.com/regressguard/regressguard/internal/snapshot"
)

// Severity levels for check results.
const (
	SeverityCritical = "CRITICAL"
	SeverityWarning  = "WARNING"
	SeverityPass     = "PASS"
)

// ResultType classifies what changed.
const (
	TypeTests   = "tests"
	TypeStatus  = "status"
	TypeSchema  = "schema"
	TypeTiming  = "timing"
)

// CheckResult is a single finding from the diff engine.
type CheckResult struct {
	Severity string `json:"severity"`
	Type     string `json:"type"`
	// Route is the canonical "METHOD /path" key, empty for test-level findings.
	Route   string `json:"route,omitempty"`
	Before  any    `json:"before,omitempty"`
	After   any    `json:"after,omitempty"`
	Message string `json:"message"`
}

// DiffResult is the full output of DiffSnapshots.
type DiffResult struct {
	Results     []CheckResult
	HasCritical bool
	HasWarning  bool
	// Counts for the summary line.
	CriticalCount int
	WarningCount  int
	PassedRoutes  int
}

// DiffSnapshots compares a before snapshot against an after snapshot and
// returns a structured DiffResult. It applies the severity rules from
// PRD Section 8.2 Module 5:
//
//   - CRITICAL: test suite newly failing, status code change, schema hash mismatch
//   - WARNING:  response time increase >200ms AND >50% of baseline
//   - PASS:     everything within acceptable variance
func DiffSnapshots(before, after snapshot.Snapshot) DiffResult {
	var results []CheckResult

	// --- Test diff (E4-T3) ---
	if after.Tests.Failed > before.Tests.Failed {
		delta := after.Tests.Failed - before.Tests.Failed
		results = append(results, CheckResult{
			Severity: SeverityCritical,
			Type:     TypeTests,
			Before:   before.Tests.Passed,
			After:    after.Tests.Passed,
			Message: fmt.Sprintf(
				"Tests: %d passed -> %d passed (%d new failure(s))",
				before.Tests.Passed, after.Tests.Passed, delta,
			),
		})
	}

	// --- Route diffs (E4-T4, E4-T5, E4-T6) ---
	passedRoutes := 0
	for key, snap := range before.Routes {
		curr, ok := after.Routes[key]
		if !ok {
			// Route disappeared — treat as critical.
			results = append(results, CheckResult{
				Severity: SeverityCritical,
				Type:     TypeStatus,
				Route:    key,
				Before:   snap.Status,
				After:    nil,
				Message:  fmt.Sprintf("%s: route no longer present in current run", key),
			})
			continue
		}

		routeCritical := false

		// E4-T4: status code change.
		if snap.Status != curr.Status {
			results = append(results, CheckResult{
				Severity: SeverityCritical,
				Type:     TypeStatus,
				Route:    key,
				Before:   snap.Status,
				After:    curr.Status,
				Message:  fmt.Sprintf("%s: status %d -> %d", key, snap.Status, curr.Status),
			})
			routeCritical = true
		}

		// E4-T5: schema hash mismatch.
		if snap.SchemaHash != curr.SchemaHash && snap.SchemaHash != "" && curr.SchemaHash != "" {
			results = append(results, CheckResult{
				Severity: SeverityCritical,
				Type:     TypeSchema,
				Route:    key,
				Before:   snap.SchemaHash[:8],
				After:    curr.SchemaHash[:8],
				Message:  fmt.Sprintf("%s: response schema changed", key),
			})
			routeCritical = true
		}

		// E4-T6: timing regression — only flag when delta >200ms AND >50% increase.
		if snap.MS > 0 {
			timingDelta := curr.MS - snap.MS
			if timingDelta > 200 && float64(timingDelta)/float64(snap.MS) > 0.5 {
				results = append(results, CheckResult{
					Severity: SeverityWarning,
					Type:     TypeTiming,
					Route:    key,
					Before:   snap.MS,
					After:    curr.MS,
					Message:  fmt.Sprintf("%s: +%dms slower (%dms -> %dms)", key, timingDelta, snap.MS, curr.MS),
				})
			}
		}

		if !routeCritical {
			passedRoutes++
		}
	}

	// Tally.
	var hasCritical, hasWarning bool
	criticalCount, warningCount := 0, 0
	for _, r := range results {
		switch r.Severity {
		case SeverityCritical:
			hasCritical = true
			criticalCount++
		case SeverityWarning:
			hasWarning = true
			warningCount++
		}
	}

	return DiffResult{
		Results:       results,
		HasCritical:   hasCritical,
		HasWarning:    hasWarning,
		CriticalCount: criticalCount,
		WarningCount:  warningCount,
		PassedRoutes:  passedRoutes,
	}
}
