package initrun

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/failures"
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

	prompted := false
	if serverURL == "" {
		if opts.Interactive {
			if err := writeInteractiveDetection(opts, detected); err != nil {
				return Result{}, err
			}
			answer, promptErr := prompt(opts, "Select dev server URL ["+DefaultServerURL+"]: ")
			if promptErr != nil {
				return Result{}, promptErr
			}
			prompted = true
			serverURL = strings.TrimSpace(answer)
			if serverURL == "" {
				serverURL = DefaultServerURL
			}
			reachable = serverReachable(opts.HTTPClient, serverURL)
		} else {
			return Result{}, failures.DevServerURLRequired()
		}
	}

	cfg := config.Config{
		Version:        1,
		ProjectRoot:    detected.Root,
		PackageManager: detected.PackageManager,
		Framework:      detected.Framework,
		TestCommand:    detected.TestCommand,
		ServerURL:      serverURL,
		Auth: config.Auth{
			Mode:       "public",
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
		answer, promptErr := prompt(opts, "Overwrite existing .regressguard/config.json? [y/N]: ")
		if promptErr != nil {
			return Result{}, promptErr
		}
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
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
	return result, writeHuman(opts, result, prompted)
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
			ui.SymbolPass + " Wrote " + result.ConfigPath,
			"",
			"Next:",
			"  " + result.Next,
		}
		for _, line := range lines {
			if _, err := fmt.Fprintln(out, line); err != nil {
				return err
			}
		}
		return nil
	}
	lines := []string{
		"RegressGuard init",
		"",
		ui.SymbolPass + " Found project root: " + result.ProjectRoot,
		ui.SymbolPass + " Detected package manager: " + result.PackageManager,
		ui.SymbolPass + " Detected framework: " + result.Framework,
		ui.SymbolPass + " Detected test command: " + result.TestCommand,
	}
	if result.ServerReachable {
		lines = append(lines, ui.SymbolPass+" Dev server reachable: "+result.ServerURL)
	} else {
		lines = append(lines, ui.SymbolWarning+" Dev server not reachable: "+result.ServerURL)
	}
	lines = append(lines,
		ui.SymbolPass+" Wrote "+result.ConfigPath,
		"",
		"Next:",
		"  "+result.Next,
	)
	for _, line := range lines {
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}
	return nil
}

func writeInteractiveDetection(opts Options, detected scanner.Detection) error {
	lines := []string{
		"RegressGuard init",
		"",
		ui.SymbolPass + " Found project root: " + detected.Root,
		ui.SymbolPass + " Detected package manager: " + detected.PackageManager,
		ui.SymbolPass + " Detected framework: " + detected.Framework,
		ui.SymbolPass + " Detected test command: " + detected.TestCommand,
		ui.SymbolWarning + " Dev server not running at " + DefaultServerURL,
		"",
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(opts.Stdout, line); err != nil {
			return err
		}
	}
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
	reader := bufio.NewReader(opts.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(text), nil
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
