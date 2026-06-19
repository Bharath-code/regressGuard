// Package engine — diff.go computes the delta between two snapshots.
// Severity rules follow PRD Section 8.2 Module 5 and Section 10.3.
package engine

import (
	"fmt"

	"github.com/Bharath-code/regressguard/internal/snapshot"
)

// Severity levels for check results.
const (
	SeverityCritical = "CRITICAL"
	SeverityWarning  = "WARNING"
	SeverityPass     = "PASS"
)

// ResultType classifies what changed.
const (
	TypeTests      = "tests"
	TypeStatus     = "status"
	TypeSchema     = "schema"
	TypeTiming     = "timing"
	TypeUnverified = "unverified"
)

// CheckResult is a single finding from the diff engine.
type CheckResult struct {
	Severity string `json:"severity"`
	Type     string `json:"type"`
	// Route is the canonical "METHOD /path" key, empty for test-level findings.
	Route        string        `json:"route,omitempty"`
	Before       any           `json:"before,omitempty"`
	After        any           `json:"after,omitempty"`
	Message      string        `json:"message"`
	// FieldChanges is populated for schema-type findings when both snapshots
	// store the normalized shape. Nil for status/timing/test findings.
	FieldChanges []FieldChange `json:"fieldChanges,omitempty"`
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
//   - CRITICAL: test suite newly failing, status code change, schema field removed/changed
//   - WARNING:  optional field added, response time increase >200ms AND >50% of baseline
//   - PASS:     everything within acceptable variance
func DiffSnapshots(before, after snapshot.Snapshot) DiffResult {
	var results []CheckResult

	// --- Test diff (E4-T3) ---
	if after.Tests.Failed > before.Tests.Failed {
		delta := after.Tests.Failed - before.Tests.Failed
		results = append(results, CheckResult{
			Severity: SeverityCritical,
			Type:     TypeTests,
			Before:   before.Tests.Failed,
			After:    after.Tests.Failed,
			Message: fmt.Sprintf(
				"Tests: %d -> %d failing (%d new failure(s))",
				before.Tests.Failed, after.Tests.Failed, delta,
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

		// P0-1: route could not be measured this run (transient timeout /
		// connection error). Report as a non-blocking WARNING and skip all
		// comparisons — a network blip must never block a commit.
		if curr.Unverified {
			reason := curr.UnverifiedReason
			if reason == "" {
				reason = "could not reach route this run"
			}
			results = append(results, CheckResult{
				Severity: SeverityWarning,
				Type:     TypeUnverified,
				Route:    key,
				Before:   snap.Status,
				After:    nil,
				Message:  fmt.Sprintf("%s: could not verify (%s) — not counted as a regression", key, reason),
			})
			continue
		}

		routeCritical := false
		routeWarning := false

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

		// E4-T5: schema hash mismatch — with field-level diff when shapes are available.
		// BUG-2: field additions are WARNING (backward-compatible), removals/changes are CRITICAL.
		if snap.SchemaHash != curr.SchemaHash && snap.SchemaHash != "" && curr.SchemaHash != "" {
			fieldChanges := DiffSchemaShapes(snap.NormalizedSchema, curr.NormalizedSchema)
			severity := classifySchemaChange(fieldChanges)
			beforeHash := snap.SchemaHash
			afterHash := curr.SchemaHash
			if len(beforeHash) > 8 {
				beforeHash = beforeHash[:8]
			}
			if len(afterHash) > 8 {
				afterHash = afterHash[:8]
			}
			results = append(results, CheckResult{
				Severity:     severity,
				Type:         TypeSchema,
				Route:        key,
				Before:       beforeHash,
				After:        afterHash,
				Message:      fmt.Sprintf("%s: response schema changed", key),
				FieldChanges: fieldChanges,
			})
			if severity == SeverityCritical {
				routeCritical = true
			} else {
				routeWarning = true
			}
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
				routeWarning = true
			}
		}

		// Only count a route as "unchanged" when it produced no finding at all —
		// a warning-only route (e.g. timing) is reported separately (P1-3).
		if !routeCritical && !routeWarning {
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

// classifySchemaChange determines the severity of a schema change based on
// the field-level diff. Per PRD Section 8.2:
//   - All changes are "added" → WARNING (backward-compatible)
//   - Any change is "removed" or "changed" → CRITICAL (breaking)
func classifySchemaChange(changes []FieldChange) string {
	if len(changes) == 0 {
		return SeverityCritical
	}
	for _, c := range changes {
		if c.Action == "removed" || c.Action == "changed" {
			return SeverityCritical
		}
	}
	return SeverityWarning
}
