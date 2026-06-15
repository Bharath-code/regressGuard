package mcprun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/snapshot"
	"github.com/mark3labs/mcp-go/mcp"
)

// --- helpers ---

func resultText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if r == nil || len(r.Content) == 0 {
		t.Fatalf("result has no content")
	}
	tc, ok := r.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content is not TextContent: %T", r.Content[0])
	}
	return tc.Text
}

func callReq(name string, args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Name: name, Arguments: args}}
}

// readAuditEntries parses every JSON line in .regressguard/mcp-audit.log.
func readAuditEntries(t *testing.T, dir string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".regressguard", "mcp-audit.log"))
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("audit line is not valid JSON: %q: %v", line, err)
		}
		entries = append(entries, e)
	}
	return entries
}

func makeTestScript(t *testing.T, dir string, passed int) string {
	t.Helper()
	script := filepath.Join(dir, "fake_test.sh")
	content := "#!/bin/sh\necho 'Tests  " + strconv.Itoa(passed) + " passed'\nexit 0\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write test script: %v", err)
	}
	return "sh " + script
}

// --- S4: path validation ---

func TestValidatePath(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"empty is allowed", "", false},
		{"root itself", root, false},
		{"file inside root", filepath.Join(root, "a", "b.json"), false},
		{"traversal escape", filepath.Join(root, "..", "..", "etc", "passwd"), true},
		{"sibling with shared prefix", root + "-evil", true}, // regression guard for the old HasPrefix bug
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePath(root, tc.path)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for %q, got nil", tc.path)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error for %q, got %v", tc.path, err)
			}
		})
	}
}

// --- S6: audit logging ---

func TestAuditLogger_writesStructuredEntry(t *testing.T) {
	dir := t.TempDir()
	a := newAuditLogger(dir)
	a.log("check", map[string]any{"since": "HEAD~1"}, "success", 42)

	entries := readAuditEntries(t, dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
	e := entries[0]
	if e["tool"] != "check" {
		t.Errorf("tool = %v, want check", e["tool"])
	}
	if e["status"] != "success" {
		t.Errorf("status = %v, want success", e["status"])
	}
	if e["durationMs"].(float64) != 42 {
		t.Errorf("durationMs = %v, want 42", e["durationMs"])
	}
	if _, ok := e["timestamp"]; !ok {
		t.Error("entry missing timestamp")
	}
	if e["args"] == nil {
		t.Error("entry missing args")
	}
}

func TestAuditLogger_appendsMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	a := newAuditLogger(dir)
	a.log("status", nil, "success", 1)
	a.log("check", nil, "error", 2)

	if got := len(readAuditEntries(t, dir)); got != 2 {
		t.Errorf("expected 2 appended entries, got %d", got)
	}
}

// --- check tool: error path (missing config) ---

func TestCheckHandler_missingConfig_returnsErrorAndAudits(t *testing.T) {
	dir := t.TempDir()
	audit := newAuditLogger(dir)
	handler := makeCheckHandler(dir, audit)

	res, err := handler(context.Background(), callReq("check", map[string]any{}))
	if err != nil {
		t.Fatalf("handler should not return a transport error, got %v", err)
	}
	if !res.IsError {
		t.Errorf("expected IsError=true when config is missing\ncontent: %s", resultText(t, res))
	}

	entries := readAuditEntries(t, dir)
	if len(entries) != 1 || entries[0]["status"] != "error" {
		t.Errorf("expected one audited error entry, got %+v", entries)
	}
}

// --- check tool: happy path ---

func TestCheckHandler_success_returnsJSONAndAudits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	testCmd := makeTestScript(t, dir, 5)
	if err := config.Write(dir, config.Config{
		Version:     1,
		TestCommand: testCmd,
		ServerURL:   srv.URL,
		Routes:      []config.Route{{Method: "GET", Path: "/api/health"}},
	}); err != nil {
		t.Fatalf("write config: %v", err)
	}
	key := snapshot.RouteKey("GET", "/api/health")
	if err := snapshot.Write(dir, snapshot.Snapshot{
		Version:   1,
		CreatedAt: time.Now(),
		Tests:     snapshot.TestSummary{Passed: 5, Failed: 0},
		Routes: map[string]snapshot.RouteRecord{
			key: {Method: "GET", Path: "/api/health", Status: 200, SchemaHash: "", MS: 20},
		},
	}); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	audit := newAuditLogger(dir)
	res, err := makeCheckHandler(dir, audit)(context.Background(), callReq("check", map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, res))
	}

	// Content must be the structured check Result as JSON.
	var parsed struct {
		Status  string `json:"status"`
		Summary struct {
			Critical int `json:"critical"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(resultText(t, res)), &parsed); err != nil {
		t.Fatalf("result content is not valid JSON: %v", err)
	}
	if parsed.Status == "" {
		t.Error("expected a non-empty status in the check result JSON")
	}

	entries := readAuditEntries(t, dir)
	if len(entries) != 1 || entries[0]["status"] != "success" || entries[0]["tool"] != "check" {
		t.Errorf("expected one audited success entry for check, got %+v", entries)
	}
}

// --- status tool: happy path (no server required) ---

func TestStatusHandler_success(t *testing.T) {
	dir := t.TempDir()
	if err := config.Write(dir, config.Config{
		Version:     1,
		TestCommand: "echo hi",
		ServerURL:   "http://localhost:3000",
		Routes:      []config.Route{{Method: "GET", Path: "/api/health"}},
	}); err != nil {
		t.Fatalf("write config: %v", err)
	}

	audit := newAuditLogger(dir)
	res, err := makeStatusHandler(dir, audit)(context.Background(), callReq("status", map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("status should succeed, got error: %s", resultText(t, res))
	}
	if !json.Valid([]byte(resultText(t, res))) {
		t.Error("status result content is not valid JSON")
	}
	if entries := readAuditEntries(t, dir); len(entries) != 1 || entries[0]["tool"] != "status" {
		t.Errorf("expected one audited status entry, got %+v", entries)
	}
}

// --- Serve: S4 root validation ---

func TestServe_invalidProjectRoot_errors(t *testing.T) {
	err := Serve(Options{Version: "test", ProjectRoot: filepath.Join(t.TempDir(), "does-not-exist")})
	if err == nil {
		t.Error("expected Serve to reject a non-existent project root")
	}
}
