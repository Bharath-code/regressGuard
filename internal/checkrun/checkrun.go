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
	"time"

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
	startTime := time.Now()
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

	// E11-T4: show spinners during check phases on TTY.
	showSpinner := !opts.JSON && !opts.Verbose && !opts.HookMode && ui.ColorEnabled(opts.Stderr)

	var testSpinner *ui.Spinner
	if showSpinner {
		testSpinner = ui.NewSpinner(opts.Stderr, "Running tests...")
		testSpinner.Start()
	} else if opts.Verbose {
		_, _ = fmt.Fprintln(opts.Stderr, ui.SymbolRunning+" Running tests...")
	}
	var testProgressWriter io.Writer
	if opts.Verbose {
		testProgressWriter = opts.Stderr
	}
	testResult, testErr := engine.RunTests(cfg.TestCommand, opts.ProjectRoot, testProgressWriter)
	if testSpinner != nil {
		if testErr != nil {
			testSpinner.StopFailed("Tests failed")
		} else {
			testLine := fmt.Sprintf("%-10s %d passed, %d failed", "Tests", testResult.Passed, testResult.Failed)
			testSpinner.StopSuccess(testLine)
		}
	}
	if testErr != nil {
		return Result{}, failures.Actionable{
			Title:       "rg check failed: test command error.",
			Cause:       testErr.Error(),
			Next:        "rg config set testCommand \"npm test\"",
			MoreContext: "rg check --help",
		}
	}

	var routeSpinner *ui.Spinner
	var routeProgress *ui.RouteProgress
	if showSpinner && len(cfg.Routes) >= 4 {
		routeInputs := make([]struct{ Method, Path string }, len(cfg.Routes))
		for i, r := range cfg.Routes {
			routeInputs[i] = struct{ Method, Path string }{r.Method, r.Path}
		}
		routeProgress = ui.NewRouteProgress(opts.Stderr, routeInputs)
		routeProgress.Start()
	} else if showSpinner && len(cfg.Routes) > 0 {
		routeSpinner = ui.NewSpinner(opts.Stderr, fmt.Sprintf("Hitting %d routes...", len(cfg.Routes)))
		routeSpinner.Start()
	} else if opts.Verbose {
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
	// Wire up live progress callback.
	if routeProgress != nil {
		hitOpts.OnRouteComplete = func(index int, result engine.RouteResult) {
			if result.Skipped {
				routeProgress.MarkSkipped(index)
			} else if result.Status >= 500 {
				routeProgress.MarkFailed(index)
			} else {
				routeProgress.MarkDone(index, result.Status, result.MS)
			}
		}
	}
	routeResults := engine.HitRoutes(cfg.Routes, hitOpts, routeProgressWriter)
	if routeProgress != nil {
		routeProgress.Stop()
	}
	if routeSpinner != nil {
		routeLine := fmt.Sprintf("%-10s %d checked", "Routes", len(routeResults))
		routeSpinner.StopSuccess(routeLine)
	}

	// E11-T4: comparing spinner.
	var diffSpinner *ui.Spinner
	if showSpinner {
		diffSpinner = ui.NewSpinner(opts.Stderr, "Comparing...")
		diffSpinner.Start()
	}

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

	// Stop the comparing spinner.
	if diffSpinner != nil {
		diffSpinner.Stop()
	}

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
	return result, writeHuman(opts.Stdout, opts.Stderr, result, diff, snap, afterSnap, gitFiles, time.Since(startTime))
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
func writeHuman(stdout, stderr io.Writer, result Result, diff engine.DiffResult, before, after snapshot.Snapshot, gitFiles []string, elapsed time.Duration) error {
	_ = stderr
	switch result.Status {
	case "critical":
		return writeHumanCritical(stdout, result, diff, gitFiles, elapsed)
	case "warning":
		return writeHumanWarning(stdout, result, diff, elapsed)
	default:
		return writeHumanPass(stdout, result, before, after, elapsed)
	}
}

// writeHumanPass renders Flow E — clean check with styled banner and celebration.
func writeHumanPass(stdout io.Writer, result Result, before, after snapshot.Snapshot, elapsed time.Duration) error {
	isTTY := ui.ColorEnabled(stdout)

	// Styled banner for the verdict.
	var header string
	if isTTY {
		header = ui.PassBanner(stdout, "PASS  No regressions detected")
	} else {
		header = ui.SymbolPass + " No regressions detected"
	}

	lines := []string{
		ui.Header(stdout, "check"),
		ui.Separator(stdout),
		"",
		header,
		"",
		ui.ResultLine(stdout, "pass", "Tests", fmt.Sprintf("%d passed, %d failed", after.Tests.Passed, after.Tests.Failed)),
		ui.ResultLine(stdout, "pass", "Routes", fmt.Sprintf("%d unchanged", result.Summary.Passed)),
		ui.ResultLine(stdout, "pass", "Timing", "within tolerance"),
		"",
		ui.Separator(stdout),
	}

	// Write lines with stagger, then celebration.
	ui.StaggeredPrint(stdout, lines)

	// Success celebration on the final line.
	ui.SuccessCelebration(stdout, paint(stdout, ui.ColorOK, "Safe to commit."))

	// Footer with timing.
	_, _ = fmt.Fprintln(stdout)
	_, _ = fmt.Fprintln(stdout, ui.Footer(stdout, elapsed))
	return nil
}

// writeHumanWarning renders Flow G — warning only with styled banner.
func writeHumanWarning(stdout io.Writer, result Result, diff engine.DiffResult, elapsed time.Duration) error {
	isTTY := ui.ColorEnabled(stdout)
	n := result.Summary.Warnings
	noun := "change"
	if n != 1 {
		noun = "changes"
	}

	// Styled banner for the verdict.
	var header string
	if isTTY {
		header = ui.WarningBanner(stdout, fmt.Sprintf("WARNING  %d non-blocking %s", n, noun))
	} else {
		header = fmt.Sprintf("%s %d non-blocking %s", ui.SymbolWarning, n, noun)
	}

	lines := []string{
		ui.Header(stdout, "check"),
		ui.Separator(stdout),
		"",
		header,
		"",
		ui.TableHeaderRow(stdout, fmtTableHeader()),
	}
	for i, r := range diff.Results {
		if r.Severity == engine.SeverityWarning {
			row := fmtWarningRow(r)
			if isTTY {
				lines = append(lines, row)
				_ = i
			} else {
				lines = append(lines, row)
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, ui.Separator(stdout))
	lines = append(lines, ui.NextSection(stdout, "rg check --verbose")...)
	lines = append(lines, "")
	lines = append(lines, paint(stdout, ui.ColorWarn, "Commit allowed."))
	lines = append(lines, "")
	lines = append(lines, ui.Footer(stdout, elapsed))

	return writeLines(stdout, lines)
}

// writeHumanCritical renders Flow F — critical regression with styled banner and animated diff.
func writeHumanCritical(stdout io.Writer, result Result, diff engine.DiffResult, gitFiles []string, elapsed time.Duration) error {
	isTTY := ui.ColorEnabled(stdout)
	n := result.Summary.Critical
	noun := "regression"
	if n != 1 {
		noun = "regressions"
	}

	// Styled banner for the verdict.
	var header string
	if isTTY {
		header = ui.CriticalBanner(stdout, fmt.Sprintf("CRITICAL  %d %s detected", n, noun))
	} else {
		header = fmt.Sprintf("%s %d %s detected", ui.SymbolCritical, n, noun)
	}

	// Header section.
	headerLines := []string{
		ui.Header(stdout, "check"),
		ui.Separator(stdout),
		"",
		header,
		"",
		ui.TableHeaderRow(stdout, fmtCriticalTableHeader()),
	}
	ui.StaggeredPrint(stdout, headerLines)

	// Animated diff table — rows slide in with delay.
	rowDelay := 60 * time.Millisecond
	for _, r := range diff.Results {
		if r.Severity == engine.SeverityCritical {
			row := fmtCriticalRow(r)
			ui.AnimatedTableRow(stdout, row, rowDelay)
			if r.Type == engine.TypeSchema && len(r.FieldChanges) > 0 {
				fieldLines := engine.FormatFieldChanges(r.FieldChanges)
				for _, fl := range fieldLines {
					var colored string
					if strings.HasPrefix(strings.TrimSpace(fl), "-") {
						colored = paint(stdout, ui.ColorFail, fl)
					} else if strings.HasPrefix(strings.TrimSpace(fl), "+") {
						colored = paint(stdout, ui.ColorOK, fl)
					} else {
						colored = paint(stdout, ui.ColorWarn, fl)
					}
					ui.AnimatedTableRow(stdout, colored, 40*time.Millisecond)
				}
			}
		}
	}

	// Footer section.
	footerLines := []string{
		"",
		ui.Separator(stdout),
		"",
		"Likely cause:",
		"  " + likelyCause(diff),
	}

	if len(gitFiles) > 0 {
		footerLines = append(footerLines, "", paint(stdout, ui.ColorMuted, "Changed files since snapshot:"))
		for _, f := range gitFiles {
			footerLines = append(footerLines, "  "+paint(stdout, ui.ColorMuted, f))
		}
	}

	footerLines = append(footerLines, ui.NextSection(stdout, "rg check --verbose", "git diff")...)
	footerLines = append(footerLines, "")
	ui.StaggeredPrint(stdout, footerLines)

	// Critical reveal for the final line.
	ui.CriticalReveal(stdout, paint(stdout, ui.ColorFail, "Commit blocked."))

	// Footer with timing.
	_, _ = fmt.Fprintln(stdout)
	_, _ = fmt.Fprintln(stdout, ui.Footer(stdout, elapsed))
	return nil
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
	// E11-T6: staggered reveal for result lines on TTY.
	ui.StaggeredPrint(w, lines)
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
