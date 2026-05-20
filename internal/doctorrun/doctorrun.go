// Package doctorrun implements rg doctor.
// It verifies config, snapshot, test command, and dev server reachability.
// Uses huh/spinner for a premium diagnostic experience on TTY.
package doctorrun

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"

	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/engine"
	"github.com/Bharath-code/regressguard/internal/snapshot"
	"github.com/Bharath-code/regressguard/internal/ui"
)

// Options configures a doctor run.
type Options struct {
	ProjectRoot string
	Stdout      io.Writer
	Stderr      io.Writer
}

// Run executes all diagnostic checks and prints results.
// Returns true if all checks pass, false if any fail.
// On TTY, uses huh/spinner for a premium diagnostic experience.
func Run(opts Options) bool {
	opts = withDefaults(opts)

	// Use huh/spinner on TTY for premium feel.
	if ui.ColorEnabled(opts.Stdout) {
		var allOK bool
		_ = spinner.New().
			Title(lipgloss.NewStyle().Foreground(lipgloss.Color("#0969DA")).Render("Running diagnostics...")).
			Action(func() {
				// Brief pause so the spinner is visible.
				time.Sleep(400 * time.Millisecond)
				allOK = runChecks(opts)
			}).
			Run()
		return allOK
	}

	return runChecks(opts)
}

// runChecks performs the actual diagnostic checks.
func runChecks(opts Options) bool {
	allOK := true

	// 1. Config check.
	if config.Exists(opts.ProjectRoot) {
		cfg, err := config.Load(opts.ProjectRoot)
		if err != nil {
			printFail(opts.Stdout, "Config", "exists but is invalid: "+err.Error())
			allOK = false
		} else {
			printPass(opts.Stdout, "Config", config.Path(opts.ProjectRoot))

			// 2. Test command check.
			if cfg.TestCommand != "" {
				cmdName := strings.Fields(cfg.TestCommand)[0]
				if _, err := exec.LookPath(cmdName); err != nil {
					printFail(opts.Stdout, "Test command", fmt.Sprintf("%q not found in PATH", cmdName))
					allOK = false
				} else {
					printPass(opts.Stdout, "Test command", cfg.TestCommand)
				}
			} else {
				printWarn(opts.Stdout, "Test command", "not configured")
				allOK = false
			}

			// 3. Dev server check.
			if cfg.ServerURL != "" {
				if engine.ServerReachable(cfg.ServerURL) {
					printPass(opts.Stdout, "Dev server", cfg.ServerURL)
				} else {
					printWarn(opts.Stdout, "Dev server", cfg.ServerURL+" not responding")
				}
			} else {
				printWarn(opts.Stdout, "Dev server", "not configured")
			}

			// 4. Routes check.
			if len(cfg.Routes) > 0 {
				printPass(opts.Stdout, "Routes", fmt.Sprintf("%d configured", len(cfg.Routes)))
			} else {
				printWarn(opts.Stdout, "Routes", "none configured — rg snapshot will skip route checks")
			}
		}
	} else {
		printFail(opts.Stdout, "Config", "not found")
		_, _ = fmt.Fprintf(opts.Stdout, "  Run: rg init\n")
		allOK = false
	}

	_, _ = fmt.Fprintln(opts.Stdout)

	// 5. Snapshot check.
	if snapshot.Exists(opts.ProjectRoot) {
		snap, err := snapshot.Load(opts.ProjectRoot)
		if err != nil {
			printFail(opts.Stdout, "Snapshot", "exists but is unreadable: "+err.Error())
			allOK = false
		} else {
			age := timeSince(snap.CreatedAt)
			printPass(opts.Stdout, "Snapshot", fmt.Sprintf("version %d, %s old, commit %s", snap.Version, age, snap.GitCommit))
		}
	} else {
		printWarn(opts.Stdout, "Snapshot", "not found — run rg snapshot before rg check")
	}

	_, _ = fmt.Fprintln(opts.Stdout)

	// 6. Git check.
	if gitAvailable(opts.ProjectRoot) {
		printPass(opts.Stdout, "Git", "available")
	} else {
		printWarn(opts.Stdout, "Git", "not available — git context in rg check will be skipped")
	}

	_, _ = fmt.Fprintln(opts.Stdout)

	if allOK {
		_, _ = fmt.Fprintln(opts.Stdout, ui.Paint(opts.Stdout, ui.ColorOK, "All checks passed.")+" Ready to use rg snapshot and rg check.")
	} else {
		_, _ = fmt.Fprintln(opts.Stdout, "Some checks failed. Fix the issues above, then rerun:")
		_, _ = fmt.Fprintln(opts.Stdout, "  "+ui.Paint(opts.Stdout, ui.ColorInfo, "rg doctor"))
	}

	return allOK
}

func printPass(w io.Writer, label, detail string) {
	sym := ui.Paint(w, ui.ColorOK, ui.SymbolPass)
	_, _ = fmt.Fprintf(w, "%s %-14s %s\n", sym, label, detail)
}

func printFail(w io.Writer, label, detail string) {
	sym := ui.Paint(w, ui.ColorFail, ui.SymbolCritical)
	_, _ = fmt.Fprintf(w, "%s %-14s %s\n", sym, label, detail)
}

func printWarn(w io.Writer, label, detail string) {
	sym := ui.Paint(w, ui.ColorWarn, ui.SymbolWarning)
	_, _ = fmt.Fprintf(w, "%s %-14s %s\n", sym, label, detail)
}

func gitAvailable(root string) bool {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

func timeSince(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// ServerReachable is a quick check used by doctor.
func serverReachable(url string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
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
