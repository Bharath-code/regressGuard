// Package snapshotrun implements the rg snapshot command.
// It orchestrates test execution, route hitting, schema normalization,
// and snapshot persistence, then renders the result following the
// RegressGuard terminal design system (Section 6, Flow D).
package snapshotrun

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/engine"
	"github.com/Bharath-code/regressguard/internal/failures"
	"github.com/Bharath-code/regressguard/internal/snapshot"
	"github.com/Bharath-code/regressguard/internal/ui"
)

// Options configures a snapshot run.
type Options struct {
	// ProjectRoot is the directory that contains .regressguard/config.json.
	// Defaults to ".".
	ProjectRoot string
	JSON        bool
	Verbose     bool
	Stdout      io.Writer
	Stderr      io.Writer
}

// Result is the machine-readable outcome of rg snapshot.
type Result struct {
	Status       string         `json:"status"`
	SnapshotPath string         `json:"snapshotPath"`
	Tests        TestSummary    `json:"tests"`
	Routes       []RouteOutcome `json:"routes"`
	Next         string         `json:"next"`
}

// TestSummary is the JSON-safe test result.
type TestSummary struct {
	Passed   int    `json:"passed"`
	Failed   int    `json:"failed"`
	Skipped  int    `json:"skipped"`
	Duration string `json:"duration"`
}

// RouteOutcome is the JSON-safe per-route result.
type RouteOutcome struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status,omitempty"`
	SchemaHash string `json:"schemaHash,omitempty"`
	MS         int64  `json:"ms,omitempty"`
	Skipped    bool   `json:"skipped,omitempty"`
	SkipReason string `json:"skipReason,omitempty"`
}

// Run executes the full snapshot pipeline and returns a Result.
// Errors are returned as failures.Actionable where possible so the CLI
// can render them consistently.
func Run(opts Options) (Result, error) {
	opts = withDefaults(opts)

	// E3-T1: load and validate config.
	cfg, err := loadConfig(opts.ProjectRoot)
	if err != nil {
		return Result{}, err
	}

	snap := snapshot.Snapshot{
		Version:   snapshot.Version,
		CreatedAt: time.Now().UTC(),
		GitCommit: snapshot.GitCommit(opts.ProjectRoot),
		Routes:    make(map[string]snapshot.RouteRecord),
	}

	// E3-T2: run tests.
	if opts.Verbose {
		_, _ = fmt.Fprintln(opts.Stderr, ui.SymbolRunning+" Running tests...")
	}
	var testProgressWriter io.Writer
	if opts.Verbose {
		testProgressWriter = opts.Stderr
	}
	testResult, testErr := engine.RunTests(cfg.TestCommand, opts.ProjectRoot, testProgressWriter)
	if testErr != nil {
		// Surface as actionable — test command may be misconfigured.
		return Result{}, failures.Actionable{
			Title:       "rg snapshot failed: test command error.",
			Cause:       testErr.Error(),
			Next:        "rg config set testCommand \"npm test\"",
			MoreContext: "rg snapshot --help",
		}
	}
	snap.Tests = snapshot.TestSummary{
		Passed:   testResult.Passed,
		Failed:   testResult.Failed,
		Skipped:  testResult.Skipped,
		Duration: testResult.Duration,
	}

	// E3-T3 / E3-T4: discover routes and hit them.
	routes := cfg.Routes
	if opts.Verbose {
		_, _ = fmt.Fprintf(opts.Stderr, ui.SymbolRunning+" Hitting %d routes...\n", len(routes))
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
	routeResults := engine.HitRoutes(routes, hitOpts, routeProgressWriter)

	captured := 0
	skipped := 0
	for _, rr := range routeResults {
		if rr.Skipped {
			skipped++
			continue
		}
		captured++
		key := snapshot.RouteKey(rr.Method, rr.Path)
		snap.Routes[key] = snapshot.RouteRecord{
			Method:     rr.Method,
			Path:       rr.Path,
			Status:     rr.Status,
			SchemaHash: rr.SchemaHash,
			MS:         rr.MS,
		}
	}

	// E3-T6: save snapshot.
	if err := snapshot.Write(opts.ProjectRoot, snap); err != nil {
		return Result{}, fmt.Errorf("save snapshot: %w", err)
	}

	// Build result.
	outcomes := make([]RouteOutcome, 0, len(routeResults))
	for _, rr := range routeResults {
		o := RouteOutcome{
			Method:     rr.Method,
			Path:       rr.Path,
			Status:     rr.Status,
			SchemaHash: rr.SchemaHash,
			MS:         rr.MS,
			Skipped:    rr.Skipped,
			SkipReason: rr.SkipReason,
		}
		outcomes = append(outcomes, o)
	}

	result := Result{
		Status:       "saved",
		SnapshotPath: snapshot.Path(opts.ProjectRoot),
		Tests: TestSummary{
			Passed:   testResult.Passed,
			Failed:   testResult.Failed,
			Skipped:  testResult.Skipped,
			Duration: fmtDuration(testResult.Duration),
		},
		Routes: outcomes,
		Next:   "rg check",
	}

	// E3-T8: JSON output.
	if opts.JSON {
		return result, writeJSON(opts.Stdout, result)
	}

	// E3-T7: human output (Flow D).
	return result, writeHuman(opts.Stdout, opts.Stderr, result, captured, skipped, testResult.Duration)
}

// loadConfig reads .regressguard/config.json from the project root.
// Returns failures.Actionable if the file is missing.
func loadConfig(root string) (config.Config, error) {
	if !config.Exists(root) {
		return config.Config{}, failures.MissingConfig()
	}
	cfg, err := config.Load(root)
	if err != nil {
		return config.Config{}, failures.Actionable{
			Title:       "rg snapshot failed: config is invalid.",
			Cause:       err.Error(),
			Next:        "rg init --yes",
			MoreContext: "rg snapshot --help",
		}
	}
	if cfg.TestCommand == "" {
		return config.Config{}, failures.MissingTestCommand()
	}
	return cfg, nil
}

// writeHuman renders the Flow D snapshot screen.
func writeHuman(stdout, stderr io.Writer, result Result, captured, skipped int, testDuration time.Duration) error {
	_ = stderr // reserved for future progress output

	lines := []string{
		"Snapshot",
		"",
	}

	// Tests line.
	testLine := fmt.Sprintf("%-10s %d passed, %d failed", "Tests", result.Tests.Passed, result.Tests.Failed)
	if result.Tests.Skipped > 0 {
		testLine += fmt.Sprintf(", %d skipped", result.Tests.Skipped)
	}
	testLine += fmt.Sprintf("   %s", fmtDuration(testDuration))
	lines = append(lines, ui.SymbolPass+" "+testLine)

	// Routes line.
	routeLine := fmt.Sprintf("%-10s %d captured", "Routes", captured)
	if skipped > 0 {
		routeLine += fmt.Sprintf(", %d skipped", skipped)
	}
	lines = append(lines, ui.SymbolPass+" "+routeLine)

	// Schemas line.
	lines = append(lines, fmt.Sprintf("%s %-10s %d hashed", ui.SymbolPass, "Schemas", captured))

	lines = append(lines,
		"",
		"Saved:",
		"  "+result.SnapshotPath,
		"",
		"Next:",
		"  Ask your AI agent to make the code change, then run:",
		"  rg check",
	)

	for _, line := range lines {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
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

func fmtDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
