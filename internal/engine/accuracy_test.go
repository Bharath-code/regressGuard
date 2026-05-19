// accuracy_test.go — E6 Accuracy Controls tests.
// These tests prove the false-positive reduction strategy from PRD Section 9.2.
package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Bharath-code/regressguard/internal/config"
)

// --- E6-T1: Dynamic field auto-ignore ---

func TestAccuracy_allDynamicKeysStripped(t *testing.T) {
	// Every key in the dynamicKeys map must be stripped from normalization.
	// This ensures schema hashes are stable across runs even when these values change.
	input := map[string]any{
		"createdAt":    "2026-05-18T12:00:00",
		"updatedAt":    "2026-05-18T12:00:00",
		"timestamp":    "2026-05-18T12:00:00",
		"deletedAt":    "2026-05-18T12:00:00",
		"id":           float64(123),
		"uuid":         "550e8400-e29b-41d4-a716-446655440000",
		"token":        "abc.def.ghi",
		"sessionId":    "sess-abc123",
		"nonce":        "nonce-xyz",
		"accessToken":  "eyJ.abc.def",
		"refreshToken": "eyJ.ghi.jkl",
		"expiresAt":    "2026-06-01T00:00:00",
		"expires_at":   "2026-06-01T00:00:00",
		"created_at":   "2026-05-18T12:00:00",
		"updated_at":   "2026-05-18T12:00:00",
		"deleted_at":   "2026-05-18T12:00:00",
		// Non-dynamic field that must survive.
		"name": "Priya",
	}

	got := Normalize(input).(map[string]any)

	for key := range dynamicKeys {
		if _, ok := got[key]; ok {
			t.Errorf("dynamic key %q should be stripped from normalized output", key)
		}
	}

	if got["name"] != "string" {
		t.Errorf("non-dynamic field 'name' should survive normalization, got %v", got["name"])
	}
}

func TestAccuracy_dynamicValuesProduceSameHash(t *testing.T) {
	// Two responses with different dynamic values but identical schema shape
	// must produce the same hash — this is the core false-positive prevention.
	body1 := []byte(`{
		"id": 1,
		"name": "Alice",
		"email": "alice@example.com",
		"createdAt": "2026-01-01T00:00:00",
		"updatedAt": "2026-01-15T00:00:00",
		"sessionId": "sess-aaa",
		"token": "eyJ.aaa.bbb"
	}`)
	body2 := []byte(`{
		"id": 999,
		"name": "Bob",
		"email": "bob@example.com",
		"createdAt": "2026-05-19T12:00:00",
		"updatedAt": "2026-05-19T12:00:00",
		"sessionId": "sess-zzz",
		"token": "eyJ.zzz.yyy"
	}`)

	h1 := NormalizeAndHash(body1, nil)
	h2 := NormalizeAndHash(body2, nil)

	if h1 != h2 {
		t.Errorf("same schema shape with different dynamic values should hash identically\nh1=%s\nh2=%s", h1, h2)
	}
}

func TestAccuracy_nonDynamicFieldChangeChangesHash(t *testing.T) {
	// Removing a non-dynamic field (like "subscription") must change the hash.
	// This is the core regression detection mechanism.
	body1 := []byte(`{"name":"Alice","subscription":"pro","plan":"annual"}`)
	body2 := []byte(`{"name":"Bob","plan":"annual"}`) // subscription removed

	h1 := NormalizeAndHash(body1, nil)
	h2 := NormalizeAndHash(body2, nil)

	if h1 == h2 {
		t.Error("removing 'subscription' field should change the schema hash")
	}
}

// --- E6-T2: User ignore rules ---

func TestAccuracy_userIgnoreRulesSuppressFields(t *testing.T) {
	// Fields listed in config.ignoreFields must be stripped from normalization,
	// preventing false positives from known-volatile application fields.
	body1 := []byte(`{"name":"Alice","internalRef":"ref-001","requestId":"req-aaa"}`)
	body2 := []byte(`{"name":"Bob","internalRef":"ref-999","requestId":"req-zzz"}`)

	ignoreFields := []string{"internalRef", "requestId"}

	h1 := NormalizeAndHash(body1, ignoreFields)
	h2 := NormalizeAndHash(body2, ignoreFields)

	if h1 != h2 {
		t.Errorf("user-ignored fields should not affect hash\nh1=%s\nh2=%s", h1, h2)
	}
}

func TestAccuracy_userIgnoreDoesNotSuppressOtherFields(t *testing.T) {
	// Ignoring one field must not suppress other fields.
	body1 := []byte(`{"name":"Alice","subscription":"pro","requestId":"req-aaa"}`)
	body2 := []byte(`{"name":"Bob","requestId":"req-zzz"}`) // subscription removed

	ignoreFields := []string{"requestId"}

	h1 := NormalizeAndHash(body1, ignoreFields)
	h2 := NormalizeAndHash(body2, ignoreFields)

	if h1 == h2 {
		t.Error("removing 'subscription' should still change hash even with other fields ignored")
	}
}

func TestAccuracy_emptyIgnoreListBehavesLikeNil(t *testing.T) {
	body := []byte(`{"name":"Alice","role":"admin"}`)
	h1 := NormalizeAndHash(body, nil)
	h2 := NormalizeAndHash(body, []string{})
	if h1 != h2 {
		t.Error("empty ignore list should produce same hash as nil")
	}
}

// --- E6-T3: Route skip rules ---

func TestAccuracy_skipListPreventsRouteFromBeingHit(t *testing.T) {
	// Routes with Skip:true must never be hit, even if the server is reachable.
	hitCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	routes := []config.Route{
		{Method: "GET", Path: "/api/public"},
		{Method: "GET", Path: "/api/admin", Skip: true},
		{Method: "GET", Path: "/api/internal", Skip: true},
	}
	opts := HitOptions{ServerURL: srv.URL, HTTPClient: srv.Client()}
	results := HitRoutes(routes, opts, nil)

	// Only /api/public should be hit.
	if hitCount != 1 {
		t.Errorf("expected 1 route hit, got %d (skip list not respected)", hitCount)
	}

	skipped := 0
	for _, r := range results {
		if r.Skipped {
			skipped++
		}
	}
	if skipped != 2 {
		t.Errorf("expected 2 skipped routes, got %d", skipped)
	}
}

func TestAccuracy_skipListSkipReasonIsInformative(t *testing.T) {
	routes := []config.Route{
		{Method: "GET", Path: "/api/admin", Skip: true},
	}
	opts := HitOptions{ServerURL: "http://localhost:9999"}
	results := HitRoutes(routes, opts, nil)

	if len(results) != 1 || !results[0].Skipped {
		t.Fatal("expected 1 skipped result")
	}
	if results[0].SkipReason == "" {
		t.Error("skipped route should have a non-empty SkipReason")
	}
}

// --- E6-T4: Auth headers ---

func TestAccuracy_bearerTokenAppliedToAllRoutes(t *testing.T) {
	var receivedHeaders []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = append(receivedHeaders, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	routes := []config.Route{
		{Method: "GET", Path: "/api/users"},
		{Method: "GET", Path: "/api/profile"},
	}
	opts := HitOptions{
		ServerURL: srv.URL,
		Auth: config.Auth{
			Mode:       "bearer",
			TestToken:  "my-test-token",
			HeaderName: "Authorization",
			Prefix:     "Bearer",
		},
		HTTPClient: srv.Client(),
	}
	HitRoutes(routes, opts, nil)

	if len(receivedHeaders) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(receivedHeaders))
	}
	for i, h := range receivedHeaders {
		if h != "Bearer my-test-token" {
			t.Errorf("route %d: expected 'Bearer my-test-token', got %q", i, h)
		}
	}
}

func TestAccuracy_cookieAuthApplied(t *testing.T) {
	var receivedCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	routes := []config.Route{{Method: "GET", Path: "/api/me"}}
	opts := HitOptions{
		ServerURL: srv.URL,
		Auth: config.Auth{
			Mode:   "cookie",
			Cookie: "session=abc123; csrf=xyz",
		},
		HTTPClient: srv.Client(),
	}
	HitRoutes(routes, opts, nil)

	if receivedCookie != "session=abc123; csrf=xyz" {
		t.Errorf("expected cookie header, got %q", receivedCookie)
	}
}

func TestAccuracy_noAuthWhenModeEmpty(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	routes := []config.Route{{Method: "GET", Path: "/api/public"}}
	opts := HitOptions{
		ServerURL:  srv.URL,
		Auth:       config.Auth{}, // no auth configured
		HTTPClient: srv.Client(),
	}
	HitRoutes(routes, opts, nil)

	if receivedAuth != "" {
		t.Errorf("expected no Authorization header when auth mode is empty, got %q", receivedAuth)
	}
}

// --- E6-T5: Timeout handling ---

func TestAccuracy_slowRouteIsSkippedNotCrashed(t *testing.T) {
	// A route that hangs longer than the timeout must be marked skipped,
	// not cause a panic or unhandled error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than our short test timeout.
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	routes := []config.Route{{Method: "GET", Path: "/api/slow"}}
	opts := HitOptions{
		ServerURL:  srv.URL,
		Timeout:    50 * time.Millisecond, // very short timeout
		HTTPClient: &http.Client{Timeout: 50 * time.Millisecond},
	}
	results := HitRoutes(routes, opts, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Skipped {
		t.Error("timed-out route should be marked as skipped")
	}
	if results[0].SkipReason == "" {
		t.Error("timed-out route should have a non-empty SkipReason")
	}
}

func TestAccuracy_unreachableServerSkipsGracefully(t *testing.T) {
	// Nothing listening on this port — must not crash.
	routes := []config.Route{{Method: "GET", Path: "/api/health"}}
	opts := HitOptions{ServerURL: "http://127.0.0.1:19998"}
	results := HitRoutes(routes, opts, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Skipped {
		t.Error("unreachable route should be marked skipped")
	}
}
