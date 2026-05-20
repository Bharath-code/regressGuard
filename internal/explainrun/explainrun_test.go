package explainrun

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/failures"
	"github.com/Bharath-code/regressguard/internal/snapshot"
)

func writeCfg(t *testing.T, dir string, cfg config.Config) {
	t.Helper()
	if err := config.Write(dir, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func writeSnap(t *testing.T, dir string, snap snapshot.Snapshot) {
	t.Helper()
	if err := snapshot.Write(dir, snap); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
}

func TestRun_unchanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"test","email":"test@example.com"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := config.Config{
		Version:   1,
		ServerURL: srv.URL,
		Routes:    []config.Route{{Method: "GET", Path: "/api/users"}},
	}
	writeCfg(t, dir, cfg)

	key := snapshot.RouteKey("GET", "/api/users")
	// Use the same schema hash that the normalizer will produce.
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Routes: map[string]snapshot.RouteRecord{
			key: {
				Method:           "GET",
				Path:             "/api/users",
				Status:           200,
				SchemaHash:       "", // empty hash means schema diff is skipped
				NormalizedSchema: []byte(`{"email":"string","name":"string"}`),
				MS:               50,
			},
		},
	})

	var stdout, stderr bytes.Buffer
	result, err := Run(Options{
		ProjectRoot: dir,
		Route:       "GET /api/users",
		Stdout:      &stdout,
		Stderr:      &stderr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "unchanged" {
		t.Errorf("expected status=unchanged, got %q", result.Status)
	}
	if result.Before.Status != 200 {
		t.Errorf("expected before status=200, got %d", result.Before.Status)
	}
	if result.After.Status != 200 {
		t.Errorf("expected after status=200, got %d", result.After.Status)
	}

	out := stdout.String()
	if !strings.Contains(out, "No differences detected") {
		t.Errorf("expected 'No differences detected' in output, got:\n%s", out)
	}
}

func TestRun_statusChange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := config.Config{
		Version:   1,
		ServerURL: srv.URL,
		Routes:    []config.Route{{Method: "GET", Path: "/api/auth"}},
	}
	writeCfg(t, dir, cfg)

	key := snapshot.RouteKey("GET", "/api/auth")
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Routes: map[string]snapshot.RouteRecord{
			key: {
				Method:     "GET",
				Path:       "/api/auth",
				Status:     200,
				SchemaHash: "abc12345",
				MS:         30,
			},
		},
	})

	var stdout, stderr bytes.Buffer
	result, err := Run(Options{
		ProjectRoot: dir,
		Route:       "GET /api/auth",
		Stdout:      &stdout,
		Stderr:      &stderr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "changed" {
		t.Errorf("expected status=changed, got %q", result.Status)
	}
	if result.After.Status != 401 {
		t.Errorf("expected after status=401, got %d", result.After.Status)
	}

	// Should have a status change in changes.
	foundStatus := false
	for _, c := range result.Changes {
		if c.Type == "status" {
			foundStatus = true
			break
		}
	}
	if !foundStatus {
		t.Error("expected a status change in changes")
	}

	out := stdout.String()
	if !strings.Contains(out, "change(s) detected") {
		t.Errorf("expected 'change(s) detected' in output, got:\n%s", out)
	}
}

func TestRun_jsonOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := config.Config{
		Version:   1,
		ServerURL: srv.URL,
		Routes:    []config.Route{{Method: "GET", Path: "/api/ping"}},
	}
	writeCfg(t, dir, cfg)

	key := snapshot.RouteKey("GET", "/api/ping")
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Routes: map[string]snapshot.RouteRecord{
			key: {
				Method:     "GET",
				Path:       "/api/ping",
				Status:     200,
				SchemaHash: "",
				MS:         10,
			},
		},
	})

	var stdout, stderr bytes.Buffer
	_, err := Run(Options{
		ProjectRoot: dir,
		Route:       "GET /api/ping",
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
	if parsed.Route != "GET /api/ping" {
		t.Errorf("expected route='GET /api/ping', got %q", parsed.Route)
	}
}

func TestRun_missingSnapshot(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, config.Config{Version: 1, ServerURL: "http://localhost:3000"})

	var stdout, stderr bytes.Buffer
	_, err := Run(Options{
		ProjectRoot: dir,
		Route:       "GET /api/users",
		Stdout:      &stdout,
		Stderr:      &stderr,
	})
	if err == nil {
		t.Fatal("expected error for missing snapshot")
	}
	if _, ok := err.(failures.Actionable); !ok {
		t.Errorf("expected failures.Actionable, got %T", err)
	}
}

func TestRun_routeNotInSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfg := config.Config{
		Version:   1,
		ServerURL: srv.URL,
		Routes:    []config.Route{{Method: "GET", Path: "/api/other"}},
	}
	writeCfg(t, dir, cfg)

	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Routes:    map[string]snapshot.RouteRecord{},
	})

	var stdout, stderr bytes.Buffer
	_, err := Run(Options{
		ProjectRoot: dir,
		Route:       "GET /api/other",
		Stdout:      &stdout,
		Stderr:      &stderr,
	})
	if err == nil {
		t.Fatal("expected error for route not in snapshot")
	}
	if !strings.Contains(err.Error(), "not found in snapshot") {
		t.Errorf("expected 'not found in snapshot' error, got: %v", err)
	}
}

func TestRun_invalidRouteFormat(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, config.Config{Version: 1, ServerURL: "http://localhost:3000"})
	writeSnap(t, dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Routes:    map[string]snapshot.RouteRecord{},
	})

	var stdout, stderr bytes.Buffer
	_, err := Run(Options{
		ProjectRoot: dir,
		Route:       "invalid",
		Stdout:      &stdout,
		Stderr:      &stderr,
	})
	if err == nil {
		t.Fatal("expected error for invalid route format")
	}
	if !strings.Contains(err.Error(), "invalid route format") {
		t.Errorf("expected 'invalid route format' error, got: %v", err)
	}
}

func TestParseRouteKey(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"GET /api/users", "GET /api/users", false},
		{"get /api/users", "GET /api/users", false},
		{"POST /api/login", "POST /api/login", false},
		{"GET api/users", "GET /api/users", false}, // auto-adds /
		{"", "", true},
		{"invalid", "", true},
	}
	for _, tt := range tests {
		got, err := parseRouteKey(tt.input)
		if tt.wantErr && err == nil {
			t.Errorf("parseRouteKey(%q) expected error", tt.input)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("parseRouteKey(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("parseRouteKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
