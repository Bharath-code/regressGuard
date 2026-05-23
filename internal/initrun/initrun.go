package initrun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/failures"
	"github.com/Bharath-code/regressguard/internal/hookrun"
	"github.com/Bharath-code/regressguard/internal/scanner"
	"github.com/Bharath-code/regressguard/internal/ui"
)

const DefaultServerURL = "http://localhost:3000"

type Options struct {
	StartDir         string
	ServerURL        string
	TestCommand      string
	Yes              bool
	JSON             bool
	Interactive      bool
	ForceInteractive bool
	Stdout           io.Writer
	Stderr           io.Writer
	Stdin            io.Reader
	HTTPClient       *http.Client
}

type Result struct {
	Status          string         `json:"status"`
	ConfigPath      string         `json:"configPath"`
	ProjectRoot     string         `json:"projectRoot"`
	PackageManager  string         `json:"packageManager"`
	Framework       string         `json:"framework"`
	TestCommand     string         `json:"testCommand"`
	ServerURL       string         `json:"serverUrl"`
	ServerReachable bool           `json:"serverReachable"`
	Routes          []config.Route `json:"routes"`
	Next            string         `json:"next"`
}

func Run(opts Options) (Result, error) {
	opts = withDefaults(opts)
	detected, err := scanner.Detect(opts.StartDir, opts.TestCommand)
	if err != nil {
		return Result{}, failures.ProjectRootMissing()
	}
	if detected.TestCommand == "" {
		return Result{}, failures.MissingTestCommand()
	}

	serverURL := strings.TrimSpace(opts.ServerURL)
	reachable := false
	if serverURL != "" {
		reachable = serverReachable(opts.HTTPClient, serverURL)
	} else {
		reachable = serverReachable(opts.HTTPClient, DefaultServerURL)
		if reachable {
			serverURL = DefaultServerURL
		}
	}

	// Interactive mode: always show the guided huh experience.
	prompted := false
	authMode := "public"

	if opts.Interactive && (!opts.Yes || opts.ForceInteractive) {
		// Show detection results with staggered reveal.
		if err := writeInteractiveDetection(opts, detected, serverURL, reachable); err != nil {
			return Result{}, err
		}

		// Show server URL selection when server not detected or forced interactive.
		if serverURL == "" || opts.ForceInteractive {
			answer, promptErr := promptServerURL(opts, serverURL)
			if promptErr != nil {
				return Result{}, promptErr
			}
			prompted = true
			serverURL = strings.TrimSpace(answer)
			if serverURL == "" {
				serverURL = DefaultServerURL
			}
			reachable = serverReachable(opts.HTTPClient, serverURL)
		}

		// Show auth mode selection in interactive mode.
		selectedAuth, authErr := promptAuthMode(opts)
		if authErr == nil && selectedAuth != "" {
			authMode = selectedAuth
		}
	} else if serverURL == "" && !opts.Interactive {
		return Result{}, failures.DevServerURLRequired()
	}

	cfg := config.Config{
		Version:        1,
		ProjectRoot:    detected.Root,
		PackageManager: detected.PackageManager,
		Framework:      detected.Framework,
		TestCommand:    detected.TestCommand,
		ServerURL:      serverURL,
		Auth: config.Auth{
			Mode:       authMode,
			HeaderName: "Authorization",
			Prefix:     "Bearer",
		},
		IgnoreFields: []string{"createdAt", "updatedAt", "timestamp", "id", "uuid", "token", "sessionId", "nonce"},
		Routes:       convertRoutes(detected.Routes),
	}

	if config.Exists(detected.Root) && !opts.Yes {
		if !opts.Interactive {
			return Result{}, failures.ConfigExists(config.Path(detected.Root))
		}
		confirmed, confirmErr := promptOverwrite(opts)
		if confirmErr != nil {
			return Result{}, confirmErr
		}
		if !confirmed {
			return Result{}, failures.ConfigExists(config.Path(detected.Root))
		}
	}

	if err := config.Write(detected.Root, cfg); err != nil {
		return Result{}, err
	}

	result := Result{
		Status:          "configured",
		ConfigPath:      config.Path(detected.Root),
		ProjectRoot:     detected.Root,
		PackageManager:  detected.PackageManager,
		Framework:       detected.Framework,
		TestCommand:     detected.TestCommand,
		ServerURL:       serverURL,
		ServerReachable: reachable,
		Routes:          cfg.Routes,
		Next:            "rg snapshot",
	}
	if opts.JSON {
		return result, writeJSON(opts.Stdout, result)
	}
	if err := writeHuman(opts, result, prompted); err != nil {
		return result, err
	}

	// W2: suggest git hook auto-install after init (interactive only, not --yes batch).
	if opts.Interactive && !opts.Yes {
		offerHookInstall(opts, detected.Root)
	}

	return result, nil
}

// promptServerURL uses huh Select when on a real TTY, falls back to basic prompt.
func promptServerURL(opts Options, currentURL string) (string, error) {
	// Check if we can use huh (real TTY with os.File stdin).
	if _, ok := opts.Stdin.(*os.File); ok && ui.IsTerminal(opts.Stdin) {
		var serverURL string
		defaultOpt := DefaultServerURL
		if currentURL != "" {
			defaultOpt = currentURL
		}
		err := huh.NewSelect[string]().
			Title("Select dev server URL").
			Options(
				huh.NewOption(defaultOpt+" (detected)", defaultOpt),
				huh.NewOption("http://localhost:5173", "http://localhost:5173"),
				huh.NewOption("http://localhost:8080", "http://localhost:8080"),
				huh.NewOption("Enter custom URL...", "custom"),
			).
			Value(&serverURL).
			WithTheme(regressGuardTheme()).
			Run()
		if err != nil {
			return "", err
		}
		if serverURL == "custom" {
			var customURL string
			err = huh.NewInput().
				Title("Enter dev server URL").
				Placeholder("http://localhost:3000").
				Value(&customURL).
				WithTheme(regressGuardTheme()).
				Run()
			if err != nil {
				return "", err
			}
			return customURL, nil
		}
		return serverURL, nil
	}

	// Fallback: basic stdin prompt for piped/test input.
	return prompt(opts, "Select dev server URL ["+DefaultServerURL+"]: ")
}

// promptAuthMode uses huh Select for auth configuration.
func promptAuthMode(opts Options) (string, error) {
	if _, ok := opts.Stdin.(*os.File); ok && ui.IsTerminal(opts.Stdin) {
		var authMode string
		err := huh.NewSelect[string]().
			Title("Configure auth?").
			Options(
				huh.NewOption("Public routes only (no auth)", "public"),
				huh.NewOption("Bearer token", "bearer"),
				huh.NewOption("Cookie header", "cookie"),
			).
			Value(&authMode).
			WithTheme(regressGuardTheme()).
			Run()
		if err != nil {
			return "public", err
		}
		return authMode, nil
	}
	return "public", nil
}

// promptOverwrite uses huh Confirm for overwrite confirmation.
func promptOverwrite(opts Options) (bool, error) {
	if _, ok := opts.Stdin.(*os.File); ok && ui.IsTerminal(opts.Stdin) {
		var confirmed bool
		err := huh.NewConfirm().
			Title("Overwrite existing config?").
			Description(".regressguard/config.json already exists").
			Affirmative("Yes, overwrite").
			Negative("No, keep existing").
			Value(&confirmed).
			WithTheme(regressGuardTheme()).
			Run()
		if err != nil {
			return false, err
		}
		return confirmed, nil
	}

	// Fallback: basic stdin prompt.
	answer, err := prompt(opts, "Overwrite existing .regressguard/config.json? [y/N]: ")
	if err != nil {
		return false, err
	}
	return strings.ToLower(strings.TrimSpace(answer)) == "y", nil
}

// regressGuardTheme returns a custom huh theme matching the RegressGuard brand.
func regressGuardTheme() *huh.Theme {
	t := huh.ThemeCharm()

	// Override with RegressGuard brand colors.
	t.Focused.Title = t.Focused.Title.
		Foreground(lipgloss.Color("#E6EDF3")).
		Bold(true)

	t.Focused.SelectSelector = t.Focused.SelectSelector.
		Foreground(lipgloss.Color("#0969DA"))

	t.Focused.SelectedOption = t.Focused.SelectedOption.
		Foreground(lipgloss.Color("#2DA44E"))

	t.Focused.Option = t.Focused.Option.
		Foreground(lipgloss.Color("#8B949E"))

	t.Focused.FocusedButton = t.Focused.FocusedButton.
		Background(lipgloss.Color("#0969DA")).
		Foreground(lipgloss.Color("#FFFFFF"))

	t.Focused.BlurredButton = t.Focused.BlurredButton.
		Background(lipgloss.Color("#30363D")).
		Foreground(lipgloss.Color("#8B949E"))

	return t
}

func withDefaults(opts Options) Options {
	if opts.StartDir == "" {
		opts.StartDir = "."
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 700 * time.Millisecond}
	}
	if opts.ForceInteractive {
		opts.Interactive = true
	}
	return opts
}

func writeHuman(opts Options, result Result, compact bool) error {
	out := opts.Stdout
	if compact {
		lines := []string{
			"",
			ui.Paint(out, ui.ColorOK, ui.SymbolPass) + " Wrote " + ui.Paint(out, ui.ColorMuted, result.ConfigPath),
			"",
			ui.Paint(out, ui.ColorBold, "Next:"),
			"  " + ui.Paint(out, ui.ColorInfo, result.Next),
		}
		ui.StaggeredPrint(out, lines)
		return nil
	}
	lines := []string{
		ui.Paint(out, ui.ColorBold, "RegressGuard init"),
		"",
		ui.Paint(out, ui.ColorOK, ui.SymbolPass) + " Found project root: " + ui.Paint(out, ui.ColorMuted, result.ProjectRoot),
		ui.Paint(out, ui.ColorOK, ui.SymbolPass) + " Detected package manager: " + result.PackageManager,
		ui.Paint(out, ui.ColorOK, ui.SymbolPass) + " Detected framework: " + result.Framework,
		ui.Paint(out, ui.ColorOK, ui.SymbolPass) + " Detected test command: " + ui.Paint(out, ui.ColorInfo, result.TestCommand),
	}
	if result.ServerReachable {
		lines = append(lines, ui.Paint(out, ui.ColorOK, ui.SymbolPass)+" Dev server reachable: "+ui.Paint(out, ui.ColorInfo, result.ServerURL))
	} else {
		lines = append(lines, ui.Paint(out, ui.ColorWarn, ui.SymbolWarning)+" Dev server not reachable: "+result.ServerURL)
	}
	lines = append(lines,
		ui.Paint(out, ui.ColorOK, ui.SymbolPass)+" Wrote "+ui.Paint(out, ui.ColorMuted, result.ConfigPath),
		"",
		ui.Paint(out, ui.ColorBold, "Next:"),
		"  "+ui.Paint(out, ui.ColorInfo, result.Next),
	)
	ui.StaggeredPrint(out, lines)
	return nil
}

func writeInteractiveDetection(opts Options, detected scanner.Detection, serverURL string, reachable bool) error {
	out := opts.Stdout
	lines := []string{
		ui.Paint(out, ui.ColorBold, "RegressGuard init"),
		"",
		ui.Paint(out, ui.ColorOK, ui.SymbolPass) + " Found project root: " + ui.Paint(out, ui.ColorMuted, detected.Root),
		ui.Paint(out, ui.ColorOK, ui.SymbolPass) + " Detected package manager: " + detected.PackageManager,
		ui.Paint(out, ui.ColorOK, ui.SymbolPass) + " Detected framework: " + detected.Framework,
		ui.Paint(out, ui.ColorOK, ui.SymbolPass) + " Detected test command: " + ui.Paint(out, ui.ColorInfo, detected.TestCommand),
	}
	if serverURL != "" && reachable {
		lines = append(lines, ui.Paint(out, ui.ColorOK, ui.SymbolPass)+" Dev server reachable: "+ui.Paint(out, ui.ColorInfo, serverURL))
	} else if serverURL != "" {
		lines = append(lines, ui.Paint(out, ui.ColorWarn, ui.SymbolWarning)+" Dev server not responding: "+serverURL)
	} else {
		lines = append(lines, ui.Paint(out, ui.ColorWarn, ui.SymbolWarning)+" Dev server not running at "+DefaultServerURL)
	}
	if len(detected.Routes) > 0 {
		lines = append(lines, ui.Paint(out, ui.ColorOK, ui.SymbolPass)+fmt.Sprintf(" Discovered %d API routes", len(detected.Routes)))
	}
	lines = append(lines, "")
	ui.StaggeredPrint(out, lines)
	return nil
}

func writeJSON(w io.Writer, result Result) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func prompt(opts Options, label string) (string, error) {
	if _, err := fmt.Fprint(opts.Stdout, label); err != nil {
		return "", err
	}
	buf := make([]byte, 1024)
	n, err := opts.Stdin.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(string(buf[:n])), nil
}

func serverReachable(client *http.Client, rawURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode < 500
}

func convertRoutes(routes []scanner.Route) []config.Route {
	out := make([]config.Route, 0, len(routes))
	for _, route := range routes {
		out = append(out, config.Route{Method: route.Method, Path: route.Path})
	}
	return out
}

// offerHookInstall prompts the user to install the git hook after init.
// Only shown in interactive mode when a .git directory exists.
func offerHookInstall(opts Options, projectRoot string) {
	// Check if .git exists (hook only makes sense in a git repo).
	gitDir := filepath.Join(projectRoot, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return
	}

	// Check if hook is already installed.
	hookPath := filepath.Join(gitDir, "hooks", "pre-commit")
	if data, err := os.ReadFile(hookPath); err == nil {
		content := string(data)
		if strings.Contains(content, "rg check") || strings.Contains(content, "regressguard") {
			return // already installed
		}
	}

	// Prompt user.
	if _, ok := opts.Stdin.(*os.File); ok && ui.IsTerminal(opts.Stdin) {
		var install bool
		err := huh.NewConfirm().
			Title("Install pre-commit hook?").
			Description("Automatically run rg check before every commit").
			Affirmative("Yes, install").
			Negative("No, skip").
			Value(&install).
			WithTheme(regressGuardTheme()).
			Run()
		if err != nil || !install {
			return
		}
	} else {
		// Non-huh fallback: just print suggestion.
		_, _ = fmt.Fprintln(opts.Stdout)
		_, _ = fmt.Fprintln(opts.Stdout, ui.Paint(opts.Stdout, ui.ColorMuted, "Protect every commit:"))
		_, _ = fmt.Fprintln(opts.Stdout, "  "+ui.Paint(opts.Stdout, ui.ColorInfo, "rg hook install"))
		return
	}

	// Install the hook.
	_, err := hookrun.Install(hookrun.InstallOptions{
		GitDir:      gitDir,
		ProjectRoot: projectRoot,
		Stdout:      opts.Stdout,
		Stderr:      opts.Stderr,
	})
	if err != nil {
		_, _ = fmt.Fprintf(opts.Stderr, "%s Hook install failed: %v\n",
			ui.Paint(opts.Stderr, ui.ColorWarn, ui.SymbolWarning), err)
	}
}
