package snapshotrun

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/regressguard/regressguard/internal/config"
	"github.com/regressguard/regressguard/internal/failures"
	"github.com/regressguard/regressguard/internal/snapshot"
)

// makeProject creates a temp directory with a minimal .regressguard/config.json.
func makeProject(t *testing.T, cfg config.Config) string {
	t.Helper()
	dir := t.TempDir()
	if err := config.Write(dir, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return dir
}

// makeTestScript writes a tiny shell script that prints a vitest-style summary.
func makeTestScript(t *testing.T, dir string, passed, failed int) string {
	t.Helper()
	script := filepath.Join(dir, "fake_test.sh")
	content := "#!/bin/sh\n"
	if failed > 0 {
		content += "echo 'Tests  " + itoa(failed) + " failed | " + itoa(passed) + " passed'\nexit 1\n"
	} else {
		content += "echo 'Tests  " + itoa(passed) + " passed'\nexit 0\n"
	}
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write test script: %v", err)
	}
	return "sh " + script
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// TestRun_missingConfig verifies E3-T1: missing config returns actionable error.
func TestRun_missingConfig(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	_, err := Run(Options{
		ProjectRoot: dir,
		Stdout:      &stdout,
		Stderr:      &stderr,
	})
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if _, ok := err.(failures.Actionable); !ok {
		t.Errorf("expected failures.Actionable, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "rg init") {
		t.Errorf("error should mention 'rg init', got: %v", err)
	}
}

// TestRun_savesSnapshot verifies E3-T6: snapshot.json is written correctly.
func TestRun_savesSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	testCmd := makeTestScript(t, dir, 5, 0)

	cfg := config.Config{
		Version:     1,
		ProjectRoot: dir,
		TestCommand: testCmd,
		ServerURL:   srv.URL,
		Routes: []config.Route{
			{Method: "GET", Path: "/api/health"},
		},
	}
	if err := config.Write(dir, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	result, err := Run(Options{
		ProjectRoot: dir,
		Stdout:      &stdout,
		Stderr:      &stderr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "saved" {
		t.Errorf("expected status 'saved', got %q", result.Status)
	}

	// Verify snapshot file exists and is valid.
	if !snapshot.Exists(dir) {
		t.Fatal("snapshot.json was not created")
	}
	snap, err := snapshot.Load(dir)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snap.Version != 1 {
		t.Errorf("expected version 1, got %d", snap.Version)
	}
	if snap.Tests.Passed != 5 {
		t.Errorf("expected 5 passed tests, got %d", snap.Tests.Passed)
	}
	key := snapshot.RouteKey("GET", "/api/health")
	rec, ok := snap.Routes[key]
	if !ok {
		t.Fatalf("expected route %q in snapshot", key)
	}
	if rec.Status != 200 {
		t.Errorf("expected status 200, got %d", rec.Status)
	}
	if rec.SchemaHash == "" {
		t.Error("expected non-empty schema hash")
	}
}

// TestRun_humanOutput verifies E3-T7: human output matches Flow D.
func TestRun_humanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	testCmd := makeTestScript(t, dir, 3, 0)
	cfg := config.Config{
		Version:     1,
		TestCommand: testCmd,
		ServerURL:   srv.URL,
		Routes:      []config.Route{{Method: "GET", Path: "/api/ping"}},
	}
	if err := config.Write(dir, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	_, err := Run(Options{ProjectRoot: dir, Stdout: &stdout})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	checks := []string{"Snapshot", "Tests", "Routes", "Schemas", "Saved:", "rg check"}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q\nGot:\n%s", want, out)
		}
	}
}

// TestRun_jsonOutput verifies E3-T8: --json produces parseable JSON on stdout only.
func TestRun_jsonOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"value"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	testCmd := makeTestScript(t, dir, 2, 0)
	cfg := config.Config{
		Version:     1,
		TestCommand: testCmd,
		ServerURL:   srv.URL,
		Routes:      []config.Route{{Method: "GET", Path: "/api/data"}},
	}
	if err := config.Write(dir, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	_, err := Run(Options{
		ProjectRoot: dir,
		JSON:        true,
		Stdout:      &stdout,
		Stderr:      &stderr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// stdout must be valid JSON.
	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nGot:\n%s", err, stdout.String())
	}
	if result.Status != "saved" {
		t.Errorf("expected status 'saved', got %q", result.Status)
	}
	if result.Next == "" {
		t.Error("expected non-empty Next field")
	}
}

// TestRun_skippedRoutesVisible verifies skipped routes appear in result.
func TestRun_skippedRoutesVisible(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	testCmd := makeTestScript(t, dir, 1, 0)
	cfg := config.Config{
		Version:     1,
		TestCommand: testCmd,
		ServerURL:   srv.URL,
		Routes: []config.Route{
			{Method: "GET", Path: "/api/public"},
			{Method: "GET", Path: "/api/admin", Skip: true},
		},
	}
	if err := config.Write(dir, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	result, err := Run(Options{ProjectRoot: dir, Stdout: &stdout})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	skippedCount := 0
	for _, r := range result.Routes {
		if r.Skipped {
			skippedCount++
		}
	}
	if skippedCount != 1 {
		t.Errorf("expected 1 skipped route, got %d", skippedCount)
	}

	// Human output should mention skipped.
	if !strings.Contains(stdout.String(), "skipped") {
		t.Error("human output should mention skipped routes")
	}
}
