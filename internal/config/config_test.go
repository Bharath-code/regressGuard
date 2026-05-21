package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIgnoreFile_fields(t *testing.T) {
	dir := t.TempDir()
	rgDir := filepath.Join(dir, DirName)
	_ = os.MkdirAll(rgDir, 0o755)

	// Write ignore file with field entries.
	content := `# Volatile fields
field:requestId
field:traceId
internalRef
`
	_ = os.WriteFile(filepath.Join(rgDir, IgnoreFile), []byte(content), 0o644)

	fields, routes := loadIgnoreFile(dir)

	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d: %v", len(fields), fields)
	}
	if fields[0] != "requestId" || fields[1] != "traceId" || fields[2] != "internalRef" {
		t.Errorf("unexpected fields: %v", fields)
	}
	if len(routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(routes))
	}
}

func TestLoadIgnoreFile_routes(t *testing.T) {
	dir := t.TempDir()
	rgDir := filepath.Join(dir, DirName)
	_ = os.MkdirAll(rgDir, 0o755)

	content := `# Skip admin routes
route:GET /api/admin/*
route:* /api/internal/*
`
	_ = os.WriteFile(filepath.Join(rgDir, IgnoreFile), []byte(content), 0o644)

	fields, routes := loadIgnoreFile(dir)

	if len(fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(fields))
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d: %v", len(routes), routes)
	}
	if routes[0] != "GET /api/admin/*" {
		t.Errorf("unexpected route[0]: %s", routes[0])
	}
	if routes[1] != "* /api/internal/*" {
		t.Errorf("unexpected route[1]: %s", routes[1])
	}
}

func TestLoadIgnoreFile_mixed(t *testing.T) {
	dir := t.TempDir()
	rgDir := filepath.Join(dir, DirName)
	_ = os.MkdirAll(rgDir, 0o755)

	content := `# Mixed ignore file
field:requestId
traceId
route:GET /api/admin/*

# Another field
field:correlationId
`
	_ = os.WriteFile(filepath.Join(rgDir, IgnoreFile), []byte(content), 0o644)

	fields, routes := loadIgnoreFile(dir)

	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d: %v", len(fields), fields)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d: %v", len(routes), routes)
	}
}

func TestLoadIgnoreFile_missing(t *testing.T) {
	dir := t.TempDir()
	fields, routes := loadIgnoreFile(dir)
	if fields != nil || routes != nil {
		t.Error("expected nil for missing ignore file")
	}
}

func TestMatchRoutePattern(t *testing.T) {
	tests := []struct {
		routeKey string
		pattern  string
		want     bool
	}{
		{"GET /api/admin/users", "GET /api/admin/*", true},
		{"POST /api/admin/users", "GET /api/admin/*", false},
		{"GET /api/users", "GET /api/admin/*", false},
		{"GET /api/internal/metrics", "* /api/internal/*", true},
		{"POST /api/internal/health", "* /api/internal/*", true},
		{"GET /api/users", "* /api/users", true},
		{"GET /api/users", "GET /api/users", true},
		{"DELETE /api/users", "DELETE /api/users", true},
		{"GET /api/users", "POST /api/users", false},
	}

	for _, tt := range tests {
		got := matchRoutePattern(tt.routeKey, tt.pattern)
		if got != tt.want {
			t.Errorf("matchRoutePattern(%q, %q) = %v, want %v", tt.routeKey, tt.pattern, got, tt.want)
		}
	}
}

func TestApplyRouteIgnores(t *testing.T) {
	routes := []Route{
		{Method: "GET", Path: "/api/users"},
		{Method: "GET", Path: "/api/admin/settings"},
		{Method: "POST", Path: "/api/internal/metrics"},
		{Method: "GET", Path: "/api/health"},
	}

	patterns := []string{
		"GET /api/admin/*",
		"* /api/internal/*",
	}

	result := applyRouteIgnores(routes, patterns)

	if result[0].Skip {
		t.Error("/api/users should not be skipped")
	}
	if !result[1].Skip {
		t.Error("/api/admin/settings should be skipped")
	}
	if !result[2].Skip {
		t.Error("/api/internal/metrics should be skipped")
	}
	if result[3].Skip {
		t.Error("/api/health should not be skipped")
	}
}

func TestMergeUnique(t *testing.T) {
	a := []string{"foo", "bar"}
	b := []string{"bar", "baz", "qux"}

	result := mergeUnique(a, b)

	if len(result) != 4 {
		t.Fatalf("expected 4 items, got %d: %v", len(result), result)
	}
	// Should be: foo, bar, baz, qux
	expected := map[string]bool{"foo": true, "bar": true, "baz": true, "qux": true}
	for _, s := range result {
		if !expected[s] {
			t.Errorf("unexpected item: %s", s)
		}
	}
}

func TestLoad_mergesIgnoreFile(t *testing.T) {
	dir := t.TempDir()
	rgDir := filepath.Join(dir, DirName)
	_ = os.MkdirAll(rgDir, 0o755)

	// Write config.
	cfg := Config{
		Version:      1,
		TestCommand:  "npm test",
		ServerURL:    "http://localhost:3000",
		IgnoreFields: []string{"createdAt", "updatedAt"},
		Routes: []Route{
			{Method: "GET", Path: "/api/users"},
			{Method: "GET", Path: "/api/admin/settings"},
		},
	}
	if err := Write(dir, cfg); err != nil {
		t.Fatal(err)
	}

	// Write ignore file.
	ignoreContent := `field:requestId
route:GET /api/admin/*
`
	_ = os.WriteFile(filepath.Join(rgDir, IgnoreFile), []byte(ignoreContent), 0o644)

	// Load and verify merge.
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Should have merged field.
	found := false
	for _, f := range loaded.IgnoreFields {
		if f == "requestId" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'requestId' in IgnoreFields, got: %v", loaded.IgnoreFields)
	}

	// Admin route should be skipped.
	if !loaded.Routes[1].Skip {
		t.Error("expected /api/admin/settings to be skipped via ignore file")
	}
	// Users route should not be skipped.
	if loaded.Routes[0].Skip {
		t.Error("/api/users should not be skipped")
	}
}
