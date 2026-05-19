package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Bharath-code/regressguard/internal/config"
)

func TestHitRoutes_basicGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"Alice","role":"admin"}`))
	}))
	defer srv.Close()

	routes := []config.Route{
		{Method: "GET", Path: "/api/user"},
	}
	opts := HitOptions{
		ServerURL:  srv.URL,
		HTTPClient: srv.Client(),
	}
	results := HitRoutes(routes, opts, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Skipped {
		t.Errorf("expected route to be hit, got skipped: %s", r.SkipReason)
	}
	if r.Status != 200 {
		t.Errorf("expected status 200, got %d", r.Status)
	}
	if r.SchemaHash == "" {
		t.Error("expected non-empty schema hash")
	}
	if r.MS < 0 {
		t.Error("expected non-negative timing")
	}
}

func TestHitRoutes_skipsConfiguredSkip(t *testing.T) {
	routes := []config.Route{
		{Method: "GET", Path: "/api/admin", Skip: true},
	}
	opts := HitOptions{ServerURL: "http://localhost:9999"}
	results := HitRoutes(routes, opts, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Skipped {
		t.Error("expected route to be skipped")
	}
}

func TestHitRoutes_skipsPostRoutes(t *testing.T) {
	routes := []config.Route{
		{Method: "POST", Path: "/api/users"},
	}
	opts := HitOptions{ServerURL: "http://localhost:9999"}
	results := HitRoutes(routes, opts, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Skipped {
		t.Error("expected POST route to be skipped (body required)")
	}
}

func TestHitRoutes_authBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	routes := []config.Route{{Method: "GET", Path: "/api/me"}}
	opts := HitOptions{
		ServerURL: srv.URL,
		Auth: config.Auth{
			Mode:       "bearer",
			TestToken:  "test-token-123",
			HeaderName: "Authorization",
			Prefix:     "Bearer",
		},
		HTTPClient: srv.Client(),
	}
	HitRoutes(routes, opts, nil)
	if gotAuth != "Bearer test-token-123" {
		t.Errorf("expected 'Bearer test-token-123', got %q", gotAuth)
	}
}

func TestHitRoutes_schemaHashStableAcrossRuns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// id and createdAt are dynamic — hash should be stable.
		_, _ = w.Write([]byte(`{"id":999,"name":"Bob","createdAt":"2026-05-19"}`))
	}))
	defer srv.Close()

	routes := []config.Route{{Method: "GET", Path: "/api/user"}}
	opts := HitOptions{ServerURL: srv.URL, HTTPClient: srv.Client()}

	r1 := HitRoutes(routes, opts, nil)
	r2 := HitRoutes(routes, opts, nil)

	if r1[0].SchemaHash != r2[0].SchemaHash {
		t.Errorf("schema hash should be stable across runs: %s != %s", r1[0].SchemaHash, r2[0].SchemaHash)
	}
}

func TestHitRoutes_unreachableServer(t *testing.T) {
	routes := []config.Route{{Method: "GET", Path: "/api/health"}}
	opts := HitOptions{ServerURL: "http://127.0.0.1:19999"} // nothing listening
	results := HitRoutes(routes, opts, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Skipped {
		t.Error("expected unreachable route to be marked skipped")
	}
}

func TestBuildURL(t *testing.T) {
	cases := []struct {
		base, path, want string
	}{
		{"http://localhost:3000", "/api/health", "http://localhost:3000/api/health"},
		{"http://localhost:3000/", "/api/health", "http://localhost:3000/api/health"},
		{"http://localhost:3000", "api/health", "http://localhost:3000/api/health"},
	}
	for _, tc := range cases {
		got := buildURL(tc.base, tc.path)
		if got != tc.want {
			t.Errorf("buildURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
	}
}
