package engine

import (
	"strings"
	"testing"
)

func TestDiffSchemaShapes_fieldRemoved(t *testing.T) {
	before := []byte(`{"name":"string","subscription":"string","plan":"string"}`)
	after := []byte(`{"name":"string","plan":"string"}`)

	changes := DiffSchemaShapes(before, after)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Field != "subscription" {
		t.Errorf("expected field 'subscription', got %q", changes[0].Field)
	}
	if changes[0].Action != "removed" {
		t.Errorf("expected action 'removed', got %q", changes[0].Action)
	}
}

func TestDiffSchemaShapes_fieldAdded(t *testing.T) {
	before := []byte(`{"name":"string"}`)
	after := []byte(`{"name":"string","internalId":"number"}`)

	changes := DiffSchemaShapes(before, after)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Field != "internalId" {
		t.Errorf("expected field 'internalId', got %q", changes[0].Field)
	}
	if changes[0].Action != "added" {
		t.Errorf("expected action 'added', got %q", changes[0].Action)
	}
}

func TestDiffSchemaShapes_fieldTypeChanged(t *testing.T) {
	before := []byte(`{"count":"string"}`)
	after := []byte(`{"count":"number"}`)

	changes := DiffSchemaShapes(before, after)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Action != "changed" {
		t.Errorf("expected action 'changed', got %q", changes[0].Action)
	}
	if changes[0].Before != "string" || changes[0].After != "number" {
		t.Errorf("expected string->number, got %s->%s", changes[0].Before, changes[0].After)
	}
}

func TestDiffSchemaShapes_noChanges(t *testing.T) {
	shape := []byte(`{"name":"string","email":"string"}`)
	changes := DiffSchemaShapes(shape, shape)
	if len(changes) != 0 {
		t.Errorf("expected 0 changes for identical shapes, got %d", len(changes))
	}
}

func TestDiffSchemaShapes_nestedFieldRemoved(t *testing.T) {
	before := []byte(`{"user":{"name":"string","role":"string"}}`)
	after := []byte(`{"user":{"name":"string"}}`)

	changes := DiffSchemaShapes(before, after)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Field != "user.role" {
		t.Errorf("expected field 'user.role', got %q", changes[0].Field)
	}
	if changes[0].Action != "removed" {
		t.Errorf("expected action 'removed', got %q", changes[0].Action)
	}
}

func TestDiffSchemaShapes_multipleChanges(t *testing.T) {
	before := []byte(`{"name":"string","subscription":"string","plan":"string"}`)
	after := []byte(`{"name":"string","plan":"string","tier":"string"}`)

	changes := DiffSchemaShapes(before, after)

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %+v", len(changes), changes)
	}

	// Should have subscription removed and tier added.
	actions := map[string]string{}
	for _, c := range changes {
		actions[c.Field] = c.Action
	}
	if actions["subscription"] != "removed" {
		t.Errorf("expected subscription removed, got %v", actions)
	}
	if actions["tier"] != "added" {
		t.Errorf("expected tier added, got %v", actions)
	}
}

func TestDiffSchemaShapes_nilInputs(t *testing.T) {
	changes := DiffSchemaShapes(nil, []byte(`{"name":"string"}`))
	if changes != nil {
		t.Error("expected nil for nil before input")
	}
	changes = DiffSchemaShapes([]byte(`{"name":"string"}`), nil)
	if changes != nil {
		t.Error("expected nil for nil after input")
	}
}

func TestDiffSchemaShapes_invalidJSON(t *testing.T) {
	changes := DiffSchemaShapes([]byte("not json"), []byte(`{"name":"string"}`))
	if changes != nil {
		t.Error("expected nil for invalid JSON")
	}
}

func TestFormatFieldChanges_output(t *testing.T) {
	changes := []FieldChange{
		{Field: "subscription", Action: "removed", Before: "string"},
		{Field: "internalId", Action: "added", After: "number"},
		{Field: "count", Action: "changed", Before: "string", After: "number"},
	}

	lines := FormatFieldChanges(changes)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "    - subscription (string, removed)" {
		t.Errorf("unexpected line[0]: %q", lines[0])
	}
	if lines[1] != "    + internalId (number, added)" {
		t.Errorf("unexpected line[1]: %q", lines[1])
	}
	if lines[2] != "    ~ count (string -> number)" {
		t.Errorf("unexpected line[2]: %q", lines[2])
	}
}

func TestFormatFieldChanges_nil(t *testing.T) {
	lines := FormatFieldChanges(nil)
	if lines != nil {
		t.Error("expected nil for nil changes")
	}
}

func TestFormatFieldChanges_complexTypes(t *testing.T) {
	before := []byte(`{"users":[{"name":"string","email":"string"}]}`)
	after := []byte(`{"users":"empty_array"}`)

	changes := DiffSchemaShapes(before, after)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	lines := FormatFieldChanges(changes)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	if strings.Contains(lines[0], "map[") {
		t.Errorf("complex type should be JSON-encoded, not Go syntax: %q", lines[0])
	}
}
