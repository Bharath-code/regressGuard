package engine

import (
	"testing"
)

func TestNormalize_primitives(t *testing.T) {
	cases := []struct {
		input any
		want  any
	}{
		{nil, "null"},
		{"hello", "string"},
		{"2026-05-18", "date"},
		{"2026-05-18T12:00:00", "date"},
		{"550e8400-e29b-41d4-a716-446655440000", "uuid"},
		{"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U", "token"},
		{float64(42), "number"},
		{true, "boolean"},
		{false, "boolean"},
		{"1.0.0", "string"},
		{"v2.1.3", "string"},
	}
	for _, tc := range cases {
		got := Normalize(tc.input)
		if got != tc.want {
			t.Errorf("Normalize(%v) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestNormalize_semverNotToken(t *testing.T) {
	versions := []string{"1.0.0", "v2.1.3", "2026.06.19"}
	for _, v := range versions {
		got := Normalize(v)
		if got == "token" {
			t.Errorf("Normalize(%q) = %q, want anything but 'token'", v, got)
		}
	}
}

func TestNormalize_realJwtIsToken(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	got := Normalize(jwt)
	if got != "token" {
		t.Errorf("Normalize(real JWT) = %q, want 'token'", got)
	}
}

func TestNormalize_requestScopedFieldsStripped(t *testing.T) {
	input := map[string]any{
		"status":         "ok",
		"requestId":      "req_abc123",
		"traceId":        "trace_xyz789",
		"correlationId":  "corr_456",
		"spanId":         "span_012",
		"parentId":       "parent_345",
		"buildId":        "build_678",
		"request_id":     "req_snake_case",
		"trace_id":       "trace_snake",
	}
	got := Normalize(input).(map[string]any)
	for _, key := range []string{"requestId", "traceId", "correlationId", "spanId", "parentId", "buildId", "request_id", "trace_id"} {
		if _, ok := got[key]; ok {
			t.Errorf("expected %q to be stripped", key)
		}
	}
	if got["status"] != "string" {
		t.Errorf("expected status='string', got %v", got["status"])
	}
}

func TestNormalize_dynamicKeysStripped(t *testing.T) {
	input := map[string]any{
		"id":        float64(123),
		"name":      "Priya",
		"createdAt": "2026-05-18T12:00:00",
		"updatedAt": "2026-05-18T12:00:00",
		"uuid":      "550e8400-e29b-41d4-a716-446655440000",
		"token":     "abc.def.ghi",
	}
	got := Normalize(input).(map[string]any)
	if _, ok := got["id"]; ok {
		t.Error("expected 'id' to be stripped")
	}
	if _, ok := got["createdAt"]; ok {
		t.Error("expected 'createdAt' to be stripped")
	}
	if _, ok := got["updatedAt"]; ok {
		t.Error("expected 'updatedAt' to be stripped")
	}
	if got["name"] != "string" {
		t.Errorf("expected name='string', got %v", got["name"])
	}
}

func TestNormalize_emptyArray(t *testing.T) {
	got := Normalize([]any{})
	if got != "empty_array" {
		t.Errorf("expected 'empty_array', got %v", got)
	}
}

func TestNormalize_arrayNormalizesFirstElement(t *testing.T) {
	input := []any{
		map[string]any{"id": float64(1), "name": "Alice"},
	}
	got := Normalize(input).([]any)
	if len(got) != 1 {
		t.Fatalf("expected 1 element, got %d", len(got))
	}
	elem := got[0].(map[string]any)
	if _, ok := elem["id"]; ok {
		t.Error("expected 'id' stripped from array element")
	}
	if elem["name"] != "string" {
		t.Errorf("expected name='string', got %v", elem["name"])
	}
}

func TestNormalizeWithIgnore_userFields(t *testing.T) {
	input := map[string]any{
		"name":         "Priya",
		"subscription": "pro",
		"internalRef":  "ref-001",
	}
	got := NormalizeWithIgnore(input, []string{"internalRef"}).(map[string]any)
	if _, ok := got["internalRef"]; ok {
		t.Error("expected 'internalRef' to be stripped by user ignore list")
	}
	if got["name"] != "string" {
		t.Errorf("expected name='string', got %v", got["name"])
	}
	if got["subscription"] != "string" {
		t.Errorf("expected subscription='string', got %v", got["subscription"])
	}
}

func TestHashSchema_deterministic(t *testing.T) {
	input := map[string]any{
		"name":  "string",
		"email": "string",
		"role":  "string",
	}
	h1 := HashSchema(input)
	h2 := HashSchema(input)
	if h1 != h2 {
		t.Errorf("HashSchema is not deterministic: %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex SHA-256, got %d chars", len(h1))
	}
}

func TestHashSchema_keyOrderIndependent(t *testing.T) {
	a := map[string]any{"name": "string", "email": "string"}
	b := map[string]any{"email": "string", "name": "string"}
	if HashSchema(a) != HashSchema(b) {
		t.Error("HashSchema should be key-order independent")
	}
}

func TestHashSchema_differentInputsDifferentHashes(t *testing.T) {
	a := map[string]any{"name": "string"}
	b := map[string]any{"name": "string", "email": "string"}
	if HashSchema(a) == HashSchema(b) {
		t.Error("different schemas should produce different hashes")
	}
}

func TestNormalizeAndHash_validJSON(t *testing.T) {
	body := []byte(`{"id":1,"name":"Priya","createdAt":"2026-05-18"}`)
	hash := NormalizeAndHash(body, nil)
	if hash == "" {
		t.Error("expected non-empty hash for valid JSON")
	}
	// Running again should produce the same hash.
	hash2 := NormalizeAndHash(body, nil)
	if hash != hash2 {
		t.Error("NormalizeAndHash is not deterministic")
	}
}

func TestNormalizeAndHash_invalidJSON(t *testing.T) {
	hash := NormalizeAndHash([]byte("not json"), nil)
	if hash != "" {
		t.Errorf("expected empty hash for invalid JSON, got %q", hash)
	}
}

func TestNormalizeAndHash_dynamicValuesStable(t *testing.T) {
	// Two responses with different dynamic values but same shape should hash identically.
	body1 := []byte(`{"id":1,"name":"Alice","createdAt":"2026-01-01"}`)
	body2 := []byte(`{"id":2,"name":"Bob","createdAt":"2026-06-15"}`)
	h1 := NormalizeAndHash(body1, nil)
	h2 := NormalizeAndHash(body2, nil)
	if h1 != h2 {
		t.Errorf("same schema shape should hash identically regardless of values: %s != %s", h1, h2)
	}
}

func TestNormalizeAndHash_schemaChangeCausesHashChange(t *testing.T) {
	// Removing a non-dynamic field should change the hash.
	body1 := []byte(`{"name":"Alice","subscription":"pro"}`)
	body2 := []byte(`{"name":"Bob"}`)
	h1 := NormalizeAndHash(body1, nil)
	h2 := NormalizeAndHash(body2, nil)
	if h1 == h2 {
		t.Error("removing a field should change the schema hash")
	}
}
