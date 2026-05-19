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
	}
	for _, tc := range cases {
		got := Normalize(tc.input)
		if got != tc.want {
			t.Errorf("Normalize(%v) = %v, want %v", tc.input, got, tc.want)
		}
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
