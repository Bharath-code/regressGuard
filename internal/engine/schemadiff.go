// schemadiff.go — field-level schema comparison.
// Compares two normalized JSON shapes and returns human-readable field changes.
package engine

import (
	"encoding/json"
	"fmt"
	"sort"
)

// FieldChange describes a single field-level change between two schemas.
type FieldChange struct {
	Field  string `json:"field"`
	Action string `json:"action"` // "removed", "added", "changed"
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

// DiffSchemaShapes compares two JSON-encoded normalized shapes and returns
// the list of field-level changes. Returns nil if shapes are identical or
// either input is unparseable.
func DiffSchemaShapes(beforeJSON, afterJSON []byte) []FieldChange {
	if len(beforeJSON) == 0 || len(afterJSON) == 0 {
		return nil
	}
	var before, after any
	if err := json.Unmarshal(beforeJSON, &before); err != nil {
		return nil
	}
	if err := json.Unmarshal(afterJSON, &after); err != nil {
		return nil
	}
	var changes []FieldChange
	diffValues("", before, after, &changes)
	return changes
}

// diffValues recursively compares two normalized values and appends changes.
func diffValues(prefix string, before, after any, changes *[]FieldChange) {
	beforeMap, beforeIsMap := before.(map[string]any)
	afterMap, afterIsMap := after.(map[string]any)

	if beforeIsMap && afterIsMap {
		diffMaps(prefix, beforeMap, afterMap, changes)
		return
	}

	// Both are arrays — compare element shape (first element only, per normalizer).
	beforeArr, beforeIsArr := before.([]any)
	afterArr, afterIsArr := after.([]any)
	if beforeIsArr && afterIsArr {
		if len(beforeArr) > 0 && len(afterArr) > 0 {
			diffValues(prefix+"[]", beforeArr[0], afterArr[0], changes)
		}
		return
	}

	// Primitive type change.
	bs := fmt.Sprintf("%v", before)
	as := fmt.Sprintf("%v", after)
	if bs != as {
		key := prefix
		if key == "" {
			key = "(root)"
		}
		*changes = append(*changes, FieldChange{
			Field:  key,
			Action: "changed",
			Before: bs,
			After:  as,
		})
	}
}

func diffMaps(prefix string, before, after map[string]any, changes *[]FieldChange) {
	// Collect all keys.
	allKeys := make(map[string]bool)
	for k := range before {
		allKeys[k] = true
	}
	for k := range after {
		allKeys[k] = true
	}

	// Sort for deterministic output.
	keys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}

		bVal, inBefore := before[k]
		aVal, inAfter := after[k]

		switch {
		case inBefore && !inAfter:
			*changes = append(*changes, FieldChange{
				Field:  fullKey,
				Action: "removed",
				Before: fmt.Sprintf("%v", bVal),
			})
		case !inBefore && inAfter:
			*changes = append(*changes, FieldChange{
				Field:  fullKey,
				Action: "added",
				After:  fmt.Sprintf("%v", aVal),
			})
		default:
			diffValues(fullKey, bVal, aVal, changes)
		}
	}
}

// FormatFieldChanges returns a compact human-readable summary of field changes.
// Each line is prefixed with - (removed), + (added), or ~ (changed).
// Returns nil if there are no changes.
func FormatFieldChanges(changes []FieldChange) []string {
	if len(changes) == 0 {
		return nil
	}
	lines := make([]string, 0, len(changes))
	for _, c := range changes {
		switch c.Action {
		case "removed":
			lines = append(lines, fmt.Sprintf("    - %s (%s, removed)", c.Field, c.Before))
		case "added":
			lines = append(lines, fmt.Sprintf("    + %s (%s, added)", c.Field, c.After))
		case "changed":
			lines = append(lines, fmt.Sprintf("    ~ %s (%s -> %s)", c.Field, c.Before, c.After))
		}
	}
	return lines
}
