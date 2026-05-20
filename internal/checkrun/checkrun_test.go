package checkrun

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/engine"
	"github.com/Bharath-code/regressguard/internal/failures"
	"github.com/Bharath-code/regressguard/internal/snapshot"
)

// --- helpers ---

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

func writeSnap(t *testing.T, dir string, snap snapshot.Snapshot) {
	t.Helper()
	if err := snapshot.Write(dir, snap); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
}

func writeCfg(t *testing.T, dir string, cfg config.Config) {
	t.Helper()
	if err := config.Write(dir, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// --- E4-T1: load snapshot ---

func TestRun_missingConfig(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	_, err := Run(Options{ProjectRoot: dir, Stdout: &stdout, Stderr: &stderr})
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

func TestRun_missingSnapshot(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, config.Config{
		Version:     1,
		TestCommand: "echo ok",
		ServerURL:   "http://localhost:3000",
	})

	var stdout, stderr bytes.Buffer
	_, err := Run(Options{ProjectRoot: dir, Stdout: &stdout, Stderr: &stderr})
	if err == nil {
		t.Fatal("expected error for missing snapshot")
	}
	if _, ok := err.(failures.Actionable); !ok {
		t.Errorf("expected failures.Actionable, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "rg snapshot") {
		t.Errorf("error should mention 'rg snapshot', got: %v", err)
	}
}

func TestRun_incompatibleSnapshotVersion(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, config.Config{
		Version:     1,
		TestCommand: "echo ok",
		ServerURL:   "http://localhost:3000",
	})
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   99, // incompatible
		CreatedAt: time.Now(),
		Routes:    map[string]snapshot.RouteRecord{},
	})

	var stdout, stderr bytes.Buffer
	_, err := Run(Options{ProjectRoot: dir, Stdout: &stdout, Stderr: &stderr})
	if err == nil {
		t.Fatal("expected error for incompatible snapshot version")
	}
	if !strings.Contains(err.Error(), "rg snapshot") {
		t.Errorf("error should suggest 'rg snapshot', got: %v", err)
	}
}

// --- E4-T7: pass screen (Flow E) ---

func TestRun_passScreen(t *testing.T) {
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
		TestCommand: testCmd,
		ServerURL:   srv.URL,
		Routes:      []config.Route{{Method: "GET", Path: "/api/health"}},
	}
	writeCfg(t, dir, cfg)

	// Snapshot with same state as the server will return.
	key := snapshot.RouteKey("GET", "/api/health")
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Tests:     snapshot.TestSummary{Passed: 5, Failed: 0},
		Routes: map[string]snapshot.RouteRecord{
			key: {Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "", MS: 20},
		},
	})

	var stdout, stderr bytes.Buffer
	result, err := Run(Options{ProjectRoot: dir, Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Status may be "pass" or "warning" depending on schema hash comparison.
	// The snapshot has an empty hash so schema diff will be skipped.
	if result.Status == "critical" {
		t.Errorf("expected pass or warning, got critical\nOutput:\n%s", stdout.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "check") {
		t.Errorf("pass screen missing 'check'\nGot:\n%s", out)
	}
}

// --- E4-T9: critical screen (Flow F) ---

func TestRun_criticalScreen_statusChange(t *testing.T) {
	// Server returns 401 but snapshot recorded 200.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	testCmd := makeTestScript(t, dir, 5, 0)

	cfg := config.Config{
		Version:     1,
		TestCommand: testCmd,
		ServerURL:   srv.URL,
		Routes:      []config.Route{{Method: "GET", Path: "/api/auth/verify"}},
	}
	writeCfg(t, dir, cfg)

	key := snapshot.RouteKey("GET", "/api/auth/verify")
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Tests:     snapshot.TestSummary{Passed: 5, Failed: 0},
		Routes: map[string]snapshot.RouteRecord{
			key: {Method: "GET", Path: "/api/auth/verify", Status: 200, SchemaHash: "abc12345", MS: 30},
		},
	})

	var stdout, stderr bytes.Buffer
	result, err := Run(Options{ProjectRoot: dir, Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "critical" {
		t.Errorf("expected status=critical, got %q", result.Status)
	}
	if result.Summary.Critical == 0 {
		t.Error("expected at least 1 critical finding")
	}

	out := stdout.String()
	for _, want := range []string{"check", "X", "regressions detected", "Commit blocked"} {
		if !strings.Contains(out, want) {
			t.Errorf("critical screen missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestRun_criticalScreen_testRegression(t *testing.T) {
	// Tests go from 5 passing to 3 passing + 2 failing.
	dir := t.TempDir()
	testCmd := makeTestScript(t, dir, 3, 2)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := config.Config{
		Version:     1,
		TestCommand: testCmd,
		ServerURL:   srv.URL,
		Routes:      []config.Route{},
	}
	writeCfg(t, dir, cfg)

	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Tests:     snapshot.TestSummary{Passed: 5, Failed: 0},
		Routes:    map[string]snapshot.RouteRecord{},
	})

	var stdout, stderr bytes.Buffer
	result, err := Run(Options{ProjectRoot: dir, Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "critical" {
		t.Errorf("expected status=critical for test regression, got %q", result.Status)
	}

	out := stdout.String()
	if !strings.Contains(out, "X") {
		t.Errorf("critical screen missing 'X' symbol\nGot:\n%s", out)
	}
	if !strings.Contains(out, "regression") {
		t.Errorf("critical screen missing 'regression'\nGot:\n%s", out)
	}
}

// --- E4-T8: warning screen (Flow G) ---

func TestRun_warningScreen_render(t *testing.T) {
	// Test the warning render path directly via writeHumanWarning.
	var stdout bytes.Buffer
	result := Result{
		Status:  "warning",
		Summary: ResultSummary{Critical: 0, Warnings: 1, Passed: 2},
		Results: []CheckFinding{
			{
				Severity: "WARNING",
				Type:     "timing",
				Route:    "GET /api/profile",
				Before:   int64(40),
				After:    int64(460),
				Message:  "GET /api/profile: +420ms slower (40ms -> 460ms)",
			},
		},
		Next: "rg check --verbose",
	}

	diff := engine.DiffResult{
		Results: []engine.CheckResult{
			{
				Severity: engine.SeverityWarning,
				Type:     engine.TypeTiming,
				Route:    "GET /api/profile",
				Before:   int64(40),
				After:    int64(460),
				Message:  "GET /api/profile: +420ms slower (40ms -> 460ms)",
			},
		},
		HasWarning:   true,
		WarningCount: 1,
		PassedRoutes: 2,
	}

	err := writeHumanWarning(&stdout, result, diff, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("writeHumanWarning error: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"check", "!", "non-blocking", "Commit allowed"} {
		if !strings.Contains(out, want) {
			t.Errorf("warning screen missing %q\nGot:\n%s", want, out)
		}
	}
}

// --- E4-T10: JSON output ---

func TestRun_jsonOutput_pass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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
	writeCfg(t, dir, cfg)

	key := snapshot.RouteKey("GET", "/api/ping")
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Tests:     snapshot.TestSummary{Passed: 3, Failed: 0},
		Routes: map[string]snapshot.RouteRecord{
			key: {Method: "GET", Path: "/api/ping", Status: 200, SchemaHash: "", MS: 10},
		},
	})

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
	var parsed Result
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nGot:\n%s", err, stdout.String())
	}

	// stderr must not contain JSON.
	if strings.Contains(stderr.String(), "{") {
		t.Errorf("stderr should not contain JSON, got: %s", stderr.String())
	}
}

func TestRun_jsonOutput_critical(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"crash"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	testCmd := makeTestScript(t, dir, 5, 0)

	cfg := config.Config{
		Version:     1,
		TestCommand: testCmd,
		ServerURL:   srv.URL,
		Routes:      []config.Route{{Method: "GET", Path: "/api/users"}},
	}
	writeCfg(t, dir, cfg)

	key := snapshot.RouteKey("GET", "/api/users")
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Tests:     snapshot.TestSummary{Passed: 5, Failed: 0},
		Routes: map[string]snapshot.RouteRecord{
			key: {Method: "GET", Path: "/api/users", Status: 200, SchemaHash: "abc12345", MS: 30},
		},
	})

	var stdout, stderr bytes.Buffer
	result, err := Run(Options{
		ProjectRoot: dir,
		JSON:        true,
		Stdout:      &stdout,
		Stderr:      &stderr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "critical" {
		t.Errorf("expected status=critical, got %q", result.Status)
	}

	// stdout must be valid JSON with correct schema.
	var parsed Result
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nGot:\n%s", err, stdout.String())
	}
	if parsed.Status != "critical" {
		t.Errorf("JSON status should be 'critical', got %q", parsed.Status)
	}
	if parsed.Summary.Critical == 0 {
		t.Error("JSON summary.critical should be > 0")
	}
	if parsed.Results == nil {
		t.Error("JSON results should not be nil")
	}
}

func TestRun_jsonOutput_verboseStaysOnStderr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
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
	writeCfg(t, dir, cfg)

	key := snapshot.RouteKey("GET", "/api/data")
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Tests:     snapshot.TestSummary{Passed: 2, Failed: 0},
		Routes: map[string]snapshot.RouteRecord{
			key: {Method: "GET", Path: "/api/data", Status: 200, SchemaHash: "", MS: 10},
		},
	})

	var stdout, stderr bytes.Buffer
	_, err := Run(Options{
		ProjectRoot: dir,
		JSON:        true,
		Verbose:     true,
		Stdout:      &stdout,
		Stderr:      &stderr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// stdout must still be valid JSON.
	var parsed Result
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON with --verbose: %v\nGot:\n%s", err, stdout.String())
	}

	// stderr should have verbose output.
	if stderr.Len() == 0 {
		t.Error("expected verbose output on stderr")
	}
}

// --- E10-T4: Snapshot age warning ---

func TestRun_snapshotAgeWarning_staleSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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
		Routes:      []config.Route{{Method: "GET", Path: "/api/health"}},
	}
	writeCfg(t, dir, cfg)

	key := snapshot.RouteKey("GET", "/api/health")
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now().Add(-3 * 24 * time.Hour), // 3 days old
		Tests:     snapshot.TestSummary{Passed: 3, Failed: 0},
		Routes: map[string]snapshot.RouteRecord{
			key: {Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "", MS: 10},
		},
	})

	var stdout, stderr bytes.Buffer
	result, err := Run(Options{ProjectRoot: dir, Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Warning should appear on stderr.
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "Snapshot is 3d old") {
		t.Errorf("expected stale snapshot warning on stderr, got: %s", stderrStr)
	}
	if !strings.Contains(stderrStr, "rg snapshot") {
		t.Errorf("expected 'rg snapshot' suggestion in warning, got: %s", stderrStr)
	}

	// Should not affect exit code — status should still be pass (not critical).
	if result.Status == "critical" {
		t.Errorf("snapshot age warning should not cause critical status, got: %s", result.Status)
	}
}

func TestRun_snapshotAgeWarning_freshSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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
		Routes:      []config.Route{{Method: "GET", Path: "/api/health"}},
	}
	writeCfg(t, dir, cfg)

	key := snapshot.RouteKey("GET", "/api/health")
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(), // fresh
		Tests:     snapshot.TestSummary{Passed: 3, Failed: 0},
		Routes: map[string]snapshot.RouteRecord{
			key: {Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "", MS: 10},
		},
	})

	var stdout, stderr bytes.Buffer
	_, err := Run(Options{ProjectRoot: dir, Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No warning should appear for fresh snapshot.
	stderrStr := stderr.String()
	if strings.Contains(stderrStr, "Snapshot is") {
		t.Errorf("should not show age warning for fresh snapshot, got: %s", stderrStr)
	}
}

func TestRun_snapshotAgeWarning_jsonMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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
		Routes:      []config.Route{{Method: "GET", Path: "/api/health"}},
	}
	writeCfg(t, dir, cfg)

	key := snapshot.RouteKey("GET", "/api/health")
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now().Add(-3 * 24 * time.Hour), // 3 days old
		Tests:     snapshot.TestSummary{Passed: 3, Failed: 0},
		Routes: map[string]snapshot.RouteRecord{
			key: {Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "", MS: 10},
		},
	})

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

	// In JSON mode, warning goes to stderr (not stdout).
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "Snapshot is 3d old") {
		t.Errorf("expected stale snapshot warning on stderr in JSON mode, got: %s", stderrStr)
	}

	// stdout must still be valid JSON (warning must not pollute stdout).
	var parsed Result
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nGot:\n%s", err, stdout.String())
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{25 * time.Hour, "1d"},
		{3 * 24 * time.Hour, "3d"},
		{7 * 24 * time.Hour, "7d"},
		{36 * time.Hour, "1d"},
	}
	for _, tt := range tests {
		got := formatAge(tt.duration)
		if got != tt.want {
			t.Errorf("formatAge(%v) = %q, want %q", tt.duration, got, tt.want)
		}
	}
}
