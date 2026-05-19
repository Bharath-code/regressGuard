// Package checkrun implements the rg check command.
// It loads the snapshot, reruns tests and routes, diffs the results,
// and renders the outcome following the RegressGuard terminal design system
// (Section 6, Flows E/F/G/H).
package checkrun

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/engine"
	"github.com/Bharath-code/regressguard/internal/failures"
	"github.com/Bharath-code/regressguard/internal/snapshot"
	"github.com/Bharath-code/regressguard/internal/ui"
)

// Options configures a check run.
type Options struct {
	// ProjectRoot is the directory that contains .regressguard/.
	// Defaults to ".".
	ProjectRoot string
	JSON        bool
	Verbose     bool
	Stdout      io.Writer
	Stderr      io.Writer
}

// Result is the machine-readable outcome of rg check.
// The JSON schema is stable across minor versions (PRD AC E4-T10).
type Result struct {
	Status  string          `json:"status"` // "pass", "warning", "critical"
	Summary ResultSummary   `json:"summary"`
	Results []CheckFinding  `json:"results"`
	Next    string          `json:"next"`
}

// ResultSummary holds the top-level counts.
type ResultSummary struct {
	Critical int `json:"critical"`
	Warnings int `json:"warnings"`
	Passed   int `json:"passed"`
}

// CheckFinding is a single regression or warning finding.
type CheckFinding struct {
	Severity string `json:"severity"`
	Type     string `json:"type"`
	Route    string `json:"route,omitempty"`
	Before   any    `json:"before,omitempty"`
	After    any    `json:"after,omitempty"`
	Message  string `json:"message"`
}

// Run executes the full check pipeline and returns a Result.
// The caller is responsible for setting the process exit code based on
// Result.Status: "critical" → exit 1, "pass"/"warning" → exit 0.
func Run(opts Options) (Result, error) {
	opts = withDefaults(opts)

	// E4-T1: load config.
	cfg, err := loadConfig(opts.ProjectRoot)
	if err != nil {
		return Result{}, err
	}

	// E4-T1: load snapshot.
	snap, err := loadSnapshot(opts.ProjectRoot)
	if err != nil {
		return Result{}, err
	}

	// E4-T2: rerun tests.
	if opts.Verbose {
		_, _ = fmt.Fprintln(opts.Stderr, ui.SymbolRunning+" Running tests...")
	}
	var testProgressWriter io.Writer
	if opts.Verbose {
		testProgressWriter = opts.Stderr
	}
	testResult, testErr := engine.RunTests(cfg.TestCommand, opts.ProjectRoot, testProgressWriter)
	if testErr != nil {
		return Result{}, failures.Actionable{
			Title:       "rg check failed: test command error.",
			Cause:       testErr.Error(),
			Next:        "rg config set testCommand \"npm test\"",
			MoreContext: "rg check --help",
		}
	}

	// E4-T2: rerun routes.
	if opts.Verbose {
		_, _ = fmt.Fprintf(opts.Stderr, ui.SymbolRunning+" Hitting %d routes...\n", len(cfg.Routes))
	}
	var routeProgressWriter io.Writer
	if opts.Verbose {
		routeProgressWriter = opts.Stderr
	}
	hitOpts := engine.HitOptions{
		ServerURL:    cfg.ServerURL,
		Auth:         cfg.Auth,
		IgnoreFields: cfg.IgnoreFields,
		Verbose:      opts.Verbose,
	}
	routeResults := engine.HitRoutes(cfg.Routes, hitOpts, routeProgressWriter)

	// Build the "after" snapshot for diffing.
	afterSnap := snapshot.Snapshot{
		Version: snapshot.Version,
		Tests: snapshot.TestSummary{
			Passed:  testResult.Passed,
			Failed:  testResult.Failed,
			Skipped: testResult.Skipped,
		},
		Routes: make(map[string]snapshot.RouteRecord),
	}
	for _, rr := range routeResults {
		if rr.Skipped {
			continue
		}
		key := snapshot.RouteKey(rr.Method, rr.Path)
		afterSnap.Routes[key] = snapshot.RouteRecord{
			Method:     rr.Method,
			Path:       rr.Path,
			Status:     rr.Status,
			SchemaHash: rr.SchemaHash,
			MS:         rr.MS,
		}
	}

	// E4-T3 through E4-T6: diff.
	diff := engine.DiffSnapshots(snap, afterSnap)

	// Build result.
	status := statusFromDiff(diff)
	findings := make([]CheckFinding, 0, len(diff.Results))
	for _, r := range diff.Results {
		findings = append(findings, CheckFinding{
			Severity: r.Severity,
			Type:     r.Type,
			Route:    r.Route,
			Before:   r.Before,
			After:    r.After,
			Message:  r.Message,
		})
	}

	result := Result{
		Status: status,
		Summary: ResultSummary{
			Critical: diff.CriticalCount,
			Warnings: diff.WarningCount,
			Passed:   diff.PassedRoutes,
		},
		Results: findings,
		Next:    nextCommand(status),
	}

	// E4-T10: JSON output.
	if opts.JSON {
		if opts.Verbose {
			_, _ = fmt.Fprintln(opts.Stderr, ui.SymbolInfo+" Run rg check --verbose for request metadata.")
		}
		return result, writeJSON(opts.Stdout, result)
	}

	// E4-T7/T8/T9: human output.
	return result, writeHuman(opts.Stdout, opts.Stderr, result, diff, snap, afterSnap)
}

// loadConfig reads .regressguard/config.json from the project root.
func loadConfig(root string) (config.Config, error) {
	if !config.Exists(root) {
		return config.Config{}, failures.Actionable{
			Title:       "rg check failed: no config found.",
			Cause:       "RegressGuard has not been initialized for this project.",
			Next:        "rg init",
			MoreContext: "rg check --help",
		}
	}
	cfg, err := config.Load(root)
	if err != nil {
		return config.Config{}, failures.Actionable{
			Title:       "rg check failed: config is invalid.",
			Cause:       err.Error(),
			Next:        "rg init --yes",
			MoreContext: "rg check --help",
		}
	}
	if cfg.TestCommand == "" {
		return config.Config{}, failures.MissingTestCommand()
	}
	return cfg, nil
}

// loadSnapshot reads .regressguard/snapshot.json from the project root.
func loadSnapshot(root string) (snapshot.Snapshot, error) {
	if !snapshot.Exists(root) {
		return snapshot.Snapshot{}, failures.MissingSnapshot()
	}
	snap, err := snapshot.Load(root)
	if err != nil {
		return snapshot.Snapshot{}, failures.Actionable{
			Title:       "rg check failed: snapshot is unreadable.",
			Cause:       err.Error(),
			Next:        "rg snapshot",
			MoreContext: "rg check --help",
		}
	}
	// Version compatibility check.
	if snap.Version != snapshot.Version {
		return snapshot.Snapshot{}, failures.Actionable{
			Title: fmt.Sprintf(
				"rg check failed: snapshot version %d is incompatible (expected %d).",
				snap.Version, snapshot.Version,
			),
			Cause:       "The snapshot was created by a different version of RegressGuard.",
			Next:        "rg snapshot",
			MoreContext: "rg check --help",
		}
	}
	return snap, nil
}

// statusFromDiff maps a DiffResult to a status string.
func statusFromDiff(diff engine.DiffResult) string {
	if diff.HasCritical {
		return "critical"
	}
	if diff.HasWarning {
		return "warning"
	}
	return "pass"
}

// nextCommand returns the suggested next action for a given status.
func nextCommand(status string) string {
	switch status {
	case "critical":
		return "rg check --verbose"
	case "warning":
		return "rg check --verbose"
	default:
		return "git commit"
	}
}

// writeHuman renders the appropriate Flow screen (E/F/G) to stdout.
func writeHuman(stdout, stderr io.Writer, result Result, diff engine.DiffResult, before, after snapshot.Snapshot) error {
	_ = stderr

	switch result.Status {
	case "critical":
		return writeHumanCritical(stdout, result, diff)
	case "warning":
		return writeHumanWarning(stdout, result, diff)
	default:
		return writeHumanPass(stdout, result, before, after)
	}
}

// writeHumanPass renders Flow E — clean check.
func writeHumanPass(stdout io.Writer, result Result, before, after snapshot.Snapshot) error {
	lines := []string{
		"Check",
		"",
		ui.SymbolPass + " No regressions detected",
		"",
	}

	// Compact counts.
	lines = append(lines,
		fmt.Sprintf("  Tests       %d passed, %d failed", after.Tests.Passed, after.Tests.Failed),
		fmt.Sprintf("  Routes      %d unchanged", result.Summary.Passed),
		"  Timing      within tolerance",
		"",
		"Safe to commit.",
	)

	return writeLines(stdout, lines)
}

// writeHumanWarning renders Flow G — warning only.
func writeHumanWarning(stdout io.Writer, result Result, diff engine.DiffResult) error {
	n := result.Summary.Warnings
	noun := "change"
	if n != 1 {
		noun = "changes"
	}

	lines := []string{
		"Check",
		"",
		fmt.Sprintf("%s %d non-blocking %s", ui.SymbolWarning, n, noun),
		"",
	}

	// Table header.
	lines = append(lines, fmtTableHeader())
	for _, r := range diff.Results {
		if r.Severity == engine.SeverityWarning {
			lines = append(lines, fmtWarningRow(r))
		}
	}

	lines = append(lines,
		"",
		"Next:",
		"  rg check --verbose",
		"",
		"Commit allowed.",
	)

	return writeLines(stdout, lines)
}

// writeHumanCritical renders Flow F — critical regression.
func writeHumanCritical(stdout io.Writer, result Result, diff engine.DiffResult) error {
	n := result.Summary.Critical
	noun := "regression"
	if n != 1 {
		noun = "regressions"
	}

	lines := []string{
		"Check",
		"",
		fmt.Sprintf("%s %d %s detected", ui.SymbolCritical, n, noun),
		"",
	}

	// Table: show critical findings.
	lines = append(lines, fmtCriticalTableHeader())
	for _, r := range diff.Results {
		if r.Severity == engine.SeverityCritical {
			lines = append(lines, fmtCriticalRow(r))
		}
	}

	// Likely cause heuristic.
	lines = append(lines, "", "Likely cause:")
	lines = append(lines, "  "+likelyCause(diff))

	lines = append(lines,
		"",
		"Next:",
		"  rg check --verbose",
		"  git diff",
		"",
		"Commit blocked.",
	)

	return writeLines(stdout, lines)
}

// fmtCriticalTableHeader returns the header row for the critical findings table.
func fmtCriticalTableHeader() string {
	return fmt.Sprintf("  %-36s  %-8s  %-8s  %s", "Route", "Before", "After", "Change")
}

// fmtCriticalRow formats a single critical finding as a table row.
func fmtCriticalRow(r engine.CheckResult) string {
	route := truncate(r.Route, 36)
	before := fmt.Sprintf("%v", r.Before)
	after := fmt.Sprintf("%v", r.After)
	change := r.Type
	return fmt.Sprintf("  %-36s  %-8s  %-8s  %s", route, before, after, change)
}

// fmtTableHeader returns the header row for the warning table.
func fmtTableHeader() string {
	return fmt.Sprintf("  %-36s  %s", "Route", "Change")
}

// fmtWarningRow formats a single warning finding as a table row.
func fmtWarningRow(r engine.CheckResult) string {
	route := truncate(r.Route, 36)
	change := r.Message
	// Trim the route prefix from the message to avoid duplication.
	if strings.HasPrefix(change, r.Route+": ") {
		change = change[len(r.Route)+2:]
	}
	return fmt.Sprintf("  %-36s  %s", route, change)
}

// likelyCause returns a human-readable heuristic for the most likely cause.
func likelyCause(diff engine.DiffResult) string {
	hasStatus := false
	hasSchema := false
	hasTests := false
	for _, r := range diff.Results {
		switch r.Type {
		case engine.TypeStatus:
			hasStatus = true
		case engine.TypeSchema:
			hasSchema = true
		case engine.TypeTests:
			hasTests = true
		}
	}
	switch {
	case hasTests && hasStatus:
		return "Test suite and API behavior changed during the last code edit."
	case hasTests:
		return "Test suite newly failing — check recent code changes."
	case hasStatus:
		return "Auth/session behavior or routing changed during the last code edit."
	case hasSchema:
		return "Response shape changed — a field may have been removed or renamed."
	default:
		return "Code behavior changed during the last edit."
	}
}

// truncate shortens a string to max n runes, appending "…" if truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "~"
}

func writeLines(w io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(w io.Writer, result Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func withDefaults(opts Options) Options {
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = "."
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return opts
}
