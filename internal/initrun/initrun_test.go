package initrun

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bharath-code/regressguard/internal/config"
)

func TestRunWritesConfigForReachableDefaultServer(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	result, err := Run(Options{
		StartDir:    root,
		ServerURL:   server.URL,
		Yes:         true,
		Stdout:      &stdout,
		Stderr:      bytes.NewBuffer(nil),
		HTTPClient:  server.Client(),
		Interactive: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ConfigPath != config.Path(root) {
		t.Fatalf("config path = %q", result.ConfigPath)
	}
	data, err := os.ReadFile(config.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid config JSON: %v\n%s", err, data)
	}
	if cfg.PackageManager != "npm" || cfg.Framework != "nextjs-app-router" || cfg.TestCommand != "npm test" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if !strings.Contains(stdout.String(), "Next:\n  rg snapshot") {
		t.Fatalf("missing next step:\n%s", stdout.String())
	}
}

func TestRunNonInteractiveRequiresServerURLWhenDefaultUnreachable(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root)

	_, err := Run(Options{
		StartDir:    root,
		Stdout:      bytes.NewBuffer(nil),
		Stderr:      bytes.NewBuffer(nil),
		Interactive: false,
	})
	if err == nil {
		t.Fatal("expected non-interactive server URL error")
	}
	if !strings.Contains(err.Error(), "rg init --server-url http://localhost:3000") {
		t.Fatalf("missing rerun command:\n%s", err.Error())
	}
}

func TestRunInteractivePromptsForServerURL(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	result, err := Run(Options{
		StartDir:         root,
		Yes:              true,
		ForceInteractive: true,
		Stdin:            strings.NewReader(server.URL + "\n"),
		Stdout:           &stdout,
		Stderr:           bytes.NewBuffer(nil),
		HTTPClient:       server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ServerURL != server.URL {
		t.Fatalf("server URL = %q, want %q", result.ServerURL, server.URL)
	}
	if !strings.Contains(stdout.String(), "Select dev server URL") {
		t.Fatalf("expected guided prompt, got:\n%s", stdout.String())
	}
}

func TestRunDoesNotOverwriteWithoutYes(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".regressguard"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.Path(root), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Run(Options{
		StartDir:    root,
		ServerURL:   "http://localhost:3000",
		Stdout:      bytes.NewBuffer(nil),
		Stderr:      bytes.NewBuffer(nil),
		Interactive: false,
	})
	if err == nil {
		t.Fatal("expected overwrite protection")
	}
	if !strings.Contains(err.Error(), "rg init --yes") {
		t.Fatalf("missing --yes next command:\n%s", err.Error())
	}
}

func TestRunJSONIsParseable(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root)
	var stdout bytes.Buffer

	_, err := Run(Options{
		StartDir:    root,
		ServerURL:   "http://localhost:3000",
		Yes:         true,
		JSON:        true,
		Stdout:      &stdout,
		Stderr:      bytes.NewBuffer(nil),
		Interactive: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if payload["status"] != "configured" {
		t.Fatalf("status = %#v", payload["status"])
	}
}

func writeProject(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{
		"scripts": {"test": "vitest run"},
		"dependencies": {"next": "^15.0.0"},
		"devDependencies": {"vitest": "^1.0.0"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	routePath := filepath.Join(root, "app", "api", "health", "route.ts")
	if err := os.MkdirAll(filepath.Dir(routePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routePath, []byte(`export async function GET() {}`), 0o644); err != nil {
		t.Fatal(err)
	}
}
