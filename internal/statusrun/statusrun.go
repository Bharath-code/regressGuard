// Package statusrun implements the rg status command.
// It provides a sub-second health check showing snapshot age, route count,
// config health, and hook status without running tests or hitting routes.
package statusrun

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/snapshot"
	"github.com/Bharath-code/regressguard/internal/ui"
)

// Options configures a status run.
type Options struct {
	ProjectRoot string
	JSON        bool
	Stdout      io.Writer
	Stderr      io.Writer
}

// Result is the machine-readable outcome of rg status.
type Result struct {
	Status        string `json:"status"`
	ConfigExists  bool   `json:"configExists"`
	Framework     string `json:"framework,omitempty"`
	TestCommand   string `json:"testCommand,omitempty"`
	ServerURL     string `json:"serverUrl,omitempty"`
	SnapshotExists bool  `json:"snapshotExists"`
	SnapshotAge   string `json:"snapshotAge,omitempty"`
	SnapshotStale bool   `json:"snapshotStale"`
	RouteCount    int    `json:"routeCount"`
	HookInstalled bool   `json:"hookInstalled"`
}

// Run executes the status check and returns a Result.
func Run(opts Options) (Result, error) {
	opts = withDefaults(opts)

	result := Result{Status: "ok"}

	// Check config.
	if !config.Exists(opts.ProjectRoot) {
		result.Status = "unconfigured"
		result.ConfigExists = false
		if opts.JSON {
			return result, writeJSON(opts.Stdout, result)
		}
		return result, writeUnconfigured(opts.Stdout)
	}
	result.ConfigExists = true

	cfg, err := config.Load(opts.ProjectRoot)
	if err == nil {
		result.Framework = cfg.Framework
		result.TestCommand = cfg.TestCommand
		result.ServerURL = cfg.ServerURL
		result.RouteCount = len(cfg.Routes)
	}

	// Check snapshot.
	if snapshot.Exists(opts.ProjectRoot) {
		result.SnapshotExists = true
		snap, snapErr := snapshot.Load(opts.ProjectRoot)
		if snapErr == nil {
			age := time.Since(snap.CreatedAt)
			result.SnapshotAge = formatAge(age)
			result.SnapshotStale = age > 24*time.Hour
		}
	}

	// Check hook.
	result.HookInstalled = hookInstalled(opts.ProjectRoot)

	if opts.JSON {
		return result, writeJSON(opts.Stdout, result)
	}
	return result, writeHuman(opts.Stdout, result)
}

func writeHuman(w io.Writer, result Result) error {
	lines := []string{
		ui.Paint(w, ui.ColorBold, "RegressGuard status"),
		"",
	}

	// Config line.
	lines = append(lines, ui.Paint(w, ui.ColorOK, ui.SymbolPass)+" Config: "+ui.Paint(w, ui.ColorMuted, result.Framework))

	// Snapshot line.
	if result.SnapshotExists {
		snapLine := "Snapshot: " + result.SnapshotAge + " old"
		if result.SnapshotStale {
			lines = append(lines, ui.Paint(w, ui.ColorWarn, ui.SymbolWarning)+" "+snapLine)
		} else {
			lines = append(lines, ui.Paint(w, ui.ColorOK, ui.SymbolPass)+" "+snapLine)
		}
	} else {
		lines = append(lines, ui.Paint(w, ui.ColorWarn, ui.SymbolWarning)+" Snapshot: none")
	}

	// Routes line.
	lines = append(lines, ui.Paint(w, ui.ColorOK, ui.SymbolPass)+fmt.Sprintf(" Routes: %d configured", result.RouteCount))

	// Hook line.
	if result.HookInstalled {
		lines = append(lines, ui.Paint(w, ui.ColorOK, ui.SymbolPass)+" Hook: installed")
	} else {
		lines = append(lines, ui.Paint(w, ui.ColorMuted, ui.SymbolSkipped)+" Hook: not installed")
	}

	// Next suggestion.
	if !result.SnapshotExists {
		lines = append(lines, "", ui.Paint(w, ui.ColorBold, "Next:"))
		lines = append(lines, "  "+ui.Paint(w, ui.ColorInfo, "rg snapshot"))
	} else if result.SnapshotStale {
		lines = append(lines, "", ui.Paint(w, ui.ColorBold, "Next:"))
		lines = append(lines, "  "+ui.Paint(w, ui.ColorInfo, "rg snapshot")+" (refresh baseline)")
	} else if !result.HookInstalled {
		lines = append(lines, "", ui.Paint(w, ui.ColorBold, "Next:"))
		lines = append(lines, "  "+ui.Paint(w, ui.ColorInfo, "rg hook install")+" (protect every commit)")
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func writeUnconfigured(w io.Writer) error {
	lines := []string{
		ui.Paint(w, ui.ColorBold, "RegressGuard status"),
		"",
		ui.Paint(w, ui.ColorWarn, ui.SymbolWarning) + " Not configured",
		"",
		ui.Paint(w, ui.ColorBold, "Next:"),
		"  " + ui.Paint(w, ui.ColorInfo, "rg init"),
	}
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

func hookInstalled(root string) bool {
	hookPath := filepath.Join(root, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return false
	}
	return len(data) > 0 && contains(string(data), "rg check") || contains(string(data), "regressguard")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func formatAge(d time.Duration) string {
	hours := int(d.Hours())
	if hours < 1 {
		mins := int(d.Minutes())
		if mins < 1 {
			return "just now"
		}
		return fmt.Sprintf("%d min", mins)
	}
	if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	}
	days := hours / 24
	return fmt.Sprintf("%dd", days)
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
