package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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

func TestHitRoutes_parallelFasterThanSequential(t *testing.T) {
	// Each route handler sleeps 200ms. With 10 routes sequentially that's 2s+.
	// With max 5 concurrency, it should complete in ~400-600ms (2 batches).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	routes := make([]config.Route, 10)
	for i := range routes {
		routes[i] = config.Route{Method: "GET", Path: fmt.Sprintf("/api/route%d", i)}
	}

	opts := HitOptions{ServerURL: srv.URL, HTTPClient: srv.Client()}

	start := time.Now()
	results := HitRoutes(routes, opts, nil)
	elapsed := time.Since(start)

	// All 10 should succeed.
	for i, r := range results {
		if r.Skipped {
			t.Errorf("route %d was skipped: %s", i, r.SkipReason)
		}
		if r.Status != 200 {
			t.Errorf("route %d status = %d, want 200", i, r.Status)
		}
	}

	// With concurrency of 5 and 200ms per route, 10 routes should take ~400-600ms.
	// Sequential would take ~2000ms. We allow up to 1500ms as a generous bound.
	if elapsed > 1500*time.Millisecond {
		t.Errorf("expected parallel completion in <1500ms, took %v (sequential would be ~2s)", elapsed)
	}
}

func TestHitRoutes_preservesOrder(t *testing.T) {
	// Routes with varying response times should still be returned in config order.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First route is slowest, last is fastest.
		switch r.URL.Path {
		case "/api/slow":
			time.Sleep(150 * time.Millisecond)
		case "/api/medium":
			time.Sleep(100 * time.Millisecond)
		case "/api/fast":
			time.Sleep(50 * time.Millisecond)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	routes := []config.Route{
		{Method: "GET", Path: "/api/slow"},
		{Method: "GET", Path: "/api/medium"},
		{Method: "GET", Path: "/api/fast"},
	}
	opts := HitOptions{ServerURL: srv.URL, HTTPClient: srv.Client()}
	results := HitRoutes(routes, opts, nil)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Order must match input regardless of completion time.
	expected := []string{"/api/slow", "/api/medium", "/api/fast"}
	for i, want := range expected {
		if results[i].Path != want {
			t.Errorf("result[%d].Path = %q, want %q", i, results[i].Path, want)
		}
	}
}

func TestServerReachable_recoveringAfterStall(t *testing.T) {
	// Simulates a dev server mid-hot-reload: the first two requests stall past
	// the probe timeout, then the server recovers. ServerReachable must retry
	// through the stall instead of reporting the server down.
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			time.Sleep(700 * time.Millisecond) // exceeds serverProbeTimeout
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if !ServerReachable(srv.URL) {
		t.Fatal("ServerReachable = false for a server that recovered mid-probe")
	}
}

func TestServerReachable_downServerFailsFast(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // nothing listening: connection refused, no timeout burned

	start := time.Now()
	if ServerReachable(srv.URL) {
		t.Fatal("ServerReachable = true for a closed server")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("down-server probe took %v, want fast refusal", elapsed)
	}
}
