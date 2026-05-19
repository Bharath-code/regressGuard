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
	"os/exec"
	"strings"

	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/engine"
	"github.com/Bharath-code/regressguard/internal/failures"
	"github.com/Bharath-code/regressguard/internal/snapshot"
	"github.com/Bharath-code/regressguard/internal/ui"
)

// Options configures a check run.
type Options struct {
	ProjectRoot string
	JSON        bool
	Verbose     bool
	HookMode    bool
	Stdout      io.Writer
	Stderr      io.Writer
}

// Result is the machine-readable outcome of rg check.
type Result struct {
	Status  string         `json:"status"`
	Summary ResultSummary  `json:"summary"`
	Results []CheckFinding `json:"results"`
	Next    string         `json:"next"`
}

// ResultSummary holds the top-level counts.
type ResultSummary struct {
	Critical int `json:"critical"`
	Warnings int `json:"warnings"`
	Passed   int `json:"passed"`
}

// CheckFinding is a single regression or warning finding.
type CheckFinding struct {
	Severity   string               `json:"severity"`
	Type       string               `json:"type"`
	Route      string               `json:"route,omitempty"`
	Before     any                  `json:"before,omitempty"`
	After      any                  `json:"after,omitempty"`
	Message    string               `json:"message"`
	SchemaDiff []engine.FieldChange `json:"schemaDiff,omitempty"`
}

// Run executes the full check pipeline and returns a Result.
func Run(opts Options) (Result, error) {
	opts = withDefaults(opts)

	if os.Getenv("RG_HOOK") == "1" {
		opts.HookMode = true
	}

	cfg, err := loadConfig(opts.ProjectRoot)
	if err != nil {
		return Result{}, err
	}

	snap, err := loadSnapshot(opts.ProjectRoot)
	if err != nil {
		return Result{}, err
	}

	// E9-T2: fast server-down detection.
	if len(cfg.Routes) > 0 && !engine.ServerReachable(cfg.ServerURL) {
		return Result{}, failures.Actionable{
			Title:       "rg check failed: dev server is not responding.",
			Cause:       "The server at " + cfg.ServerURL + " did not respond within 500ms.",
			Next:        "npm run dev",
			MoreContext: "rg doctor",
		}
	}

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
			Method:           rr.Method,
			Path:             rr.Path,
			Status:           rr.Status,
			SchemaHash:       rr.SchemaHash,
			NormalizedSchema: rr.NormalizedSchema,
			MS:               rr.MS,
		}
	}

	diff := engine.DiffSnapshots(snap, afterSnap)

	status := statusFromDiff(diff)
	findings := make([]CheckFinding, 0, len(diff.Results))
	for _, r := range diff.Results {
		findings = append(findings, CheckFinding{
			Severity:   r.Severity,
			Type:       r.Type,
			Route:      r.Route,
			Before:     r.Before,
			After:      r.After,
			Message:    r.Message,
			SchemaDiff: r.FieldChanges,
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

	if opts.JSON {
		if opts.Verbose {
			_, _ = fmt.Fprintln(opts.Stderr, ui.SymbolInfo+" Run rg check --verbose for request metadata.")
		}
		return result, writeJSON(opts.Stdout, result)
	}

	if opts.HookMode {
		return result, writeHook(opts.Stdout, result, diff)
	}

	gitFiles := gitChangedFiles(opts.ProjectRoot, snap.GitCommit)
	return result, writeHuman(opts.Stdout, opts.Stderr, result, diff, snap, afterSnap, gitFiles)
}

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

func gitChangedFiles(root, sinceCommit string) []string {
	if sinceCommit == "" || sinceCommit == "unknown" {
		return nil
	}
	cmd := exec.Command("git", "-C", root, "diff", "--name-only", sinceCommit)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, l := range lines {
		if l != "" {
			files = append(files, l)
		}
		if len(files) == 5 {
			break
		}
	}
	return files
}

func statusFromDiff(diff engine.DiffResult) string {
	if diff.HasCritical {
		return "critical"
	}
	if diff.HasWarning {
		return "warning"
	}
	return "pass"
}

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

// --- colored output helpers ---

// paint applies color only when the writer supports it.
func paint(w io.Writer, color ui.Color, text string) string {
	return ui.Paint(w, color, text)
}

// writeHuman renders the appropriate Flow screen (E/F/G) to stdout.
func writeHuman(stdout, stderr io.Writer, result Result, diff engine.DiffResult, before, after snapshot.Snapshot, gitFiles []string) error {
	_ = stderr
	switch result.Status {
	case "critical":
		return writeHumanCritical(stdout, result, diff, gitFiles)
	case "warning":
		return writeHumanWarning(stdout, result, diff)
	default:
		return writeHumanPass(stdout, result, before, after)
	}
}

// writeHumanPass renders Flow E — clean check (green).
func writeHumanPass(stdout io.Writer, result Result, before, after snapshot.Snapshot) error {
	lines := []string{
		paint(stdout, ui.ColorBold, "Check"),
		"",
		paint(stdout, ui.ColorOK, ui.SymbolPass+" No regressions detected"),
		"",
		fmt.Sprintf("  Tests       %d passed, %d failed", after.Tests.Passed, after.Tests.Failed),
		fmt.Sprintf("  Routes      %d unchanged", result.Summary.Passed),
		"  Timing      within tolerance",
		"",
		paint(stdout, ui.ColorOK, "Safe to commit."),
	}
	return writeLines(stdout, lines)
}

// writeHumanWarning renders Flow G — warning only (yellow).
func writeHumanWarning(stdout io.Writer, result Result, diff engine.DiffResult) error {
	n := result.Summary.Warnings
	noun := "change"
	if n != 1 {
		noun = "changes"
	}

	lines := []string{
		paint(stdout, ui.ColorBold, "Check"),
		"",
		paint(stdout, ui.ColorWarn, fmt.Sprintf("%s %d non-blocking %s", ui.SymbolWarning, n, noun)),
		"",
		paint(stdout, ui.ColorMuted, fmtTableHeader()),
	}
	for _, r := range diff.Results {
		if r.Severity == engine.SeverityWarning {
			lines = append(lines, fmtWarningRow(r))
		}
	}

	lines = append(lines,
		"",
		"Next:",
		"  "+paint(stdout, ui.ColorInfo, "rg check --verbose"),
		"",
		paint(stdout, ui.ColorWarn, "Commit allowed."),
	)
	return writeLines(stdout, lines)
}

// writeHumanCritical renders Flow F — critical regression (red).
func writeHumanCritical(stdout io.Writer, result Result, diff engine.DiffResult, gitFiles []string) error {
	n := result.Summary.Critical
	noun := "regression"
	if n != 1 {
		noun = "regressions"
	}

	lines := []string{
		paint(stdout, ui.ColorBold, "Check"),
		"",
		paint(stdout, ui.ColorFail, fmt.Sprintf("%s %d %s detected", ui.SymbolCritical, n, noun)),
		"",
		paint(stdout, ui.ColorMuted, fmtCriticalTableHeader()),
	}

	for _, r := range diff.Results {
		if r.Severity == engine.SeverityCritical {
			lines = append(lines, fmtCriticalRow(r))
			if r.Type == engine.TypeSchema && len(r.FieldChanges) > 0 {
				fieldLines := engine.FormatFieldChanges(r.FieldChanges)
				for _, fl := range fieldLines {
					if strings.HasPrefix(strings.TrimSpace(fl), "-") {
						lines = append(lines, paint(stdout, ui.ColorFail, fl))
					} else if strings.HasPrefix(strings.TrimSpace(fl), "+") {
						lines = append(lines, paint(stdout, ui.ColorOK, fl))
					} else {
						lines = append(lines, paint(stdout, ui.ColorWarn, fl))
					}
				}
			}
		}
	}

	lines = append(lines, "", "Likely cause:")
	lines = append(lines, "  "+likelyCause(diff))

	if len(gitFiles) > 0 {
		lines = append(lines, "", paint(stdout, ui.ColorMuted, "Changed files since snapshot:"))
		for _, f := range gitFiles {
			lines = append(lines, "  "+paint(stdout, ui.ColorMuted, f))
		}
	}

	lines = append(lines,
		"",
		"Next:",
		"  "+paint(stdout, ui.ColorInfo, "rg check --verbose"),
		"  "+paint(stdout, ui.ColorInfo, "git diff"),
		"",
		paint(stdout, ui.ColorFail, "Commit blocked."),
	)
	return writeLines(stdout, lines)
}

// writeHook renders compact Flow I output for git pre-commit hooks.
func writeHook(stdout io.Writer, result Result, diff engine.DiffResult) error {
	if result.Status == "pass" || result.Status == "warning" {
		return nil
	}

	n := result.Summary.Critical
	noun := "regression"
	if n != 1 {
		noun = "regressions"
	}

	lines := []string{
		paint(stdout, ui.ColorBold, "RegressGuard pre-commit"),
		"",
		paint(stdout, ui.ColorFail, fmt.Sprintf("%s %d %s detected", ui.SymbolCritical, n, noun)),
	}

	for _, r := range diff.Results {
		if r.Severity == engine.SeverityCritical {
			lines = append(lines, "  "+r.Message)
			break
		}
	}

	lines = append(lines,
		"",
		"Run:",
		"  "+paint(stdout, ui.ColorInfo, "rg check --verbose"),
		"",
		paint(stdout, ui.ColorFail, "Commit blocked.")+" Use --no-verify only if you accept the risk.",
	)
	return writeLines(stdout, lines)
}

func fmtCriticalTableHeader() string {
	return fmt.Sprintf("  %-36s  %-8s  %-8s  %s", "Route", "Before", "After", "Change")
}

func fmtCriticalRow(r engine.CheckResult) string {
	route := truncate(r.Route, 36)
	before := fmt.Sprintf("%v", r.Before)
	after := fmt.Sprintf("%v", r.After)
	change := r.Type
	return fmt.Sprintf("  %-36s  %-8s  %-8s  %s", route, before, after, change)
}

func fmtTableHeader() string {
	return fmt.Sprintf("  %-36s  %s", "Route", "Change")
}

func fmtWarningRow(r engine.CheckResult) string {
	route := truncate(r.Route, 36)
	change := r.Message
	if strings.HasPrefix(change, r.Route+": ") {
		change = change[len(r.Route)+2:]
	}
	return fmt.Sprintf("  %-36s  %s", route, change)
}

func likelyCause(diff engine.DiffResult) string {
	hasStatus, hasSchema, hasTests := false, false, false
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
