// Package engine contains the core logic for schema normalization, test running,
// route hitting, and diff computation.
package engine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// dynamicKeys are stripped during normalization so that schema hashes remain
// stable across runs even when values change.
var dynamicKeys = map[string]bool{
	"createdAt":    true,
	"updatedAt":    true,
	"timestamp":    true,
	"deletedAt":    true,
	"id":           true,
	"uuid":         true,
	"token":        true,
	"sessionId":    true,
	"nonce":        true,
	"accessToken":  true,
	"refreshToken": true,
	"expiresAt":    true,
	"expires_at":   true,
	"created_at":   true,
	"updated_at":   true,
	"deleted_at":   true,
}

var (
	reISO8601 = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}:\d{2})?`)
	reUUID    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	reJWT     = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)
)

// Normalize converts a parsed JSON value into a stable type-shape representation.
// Dynamic values (timestamps, UUIDs, tokens) are replaced with type labels.
// The result is deterministic and suitable for hashing.
func Normalize(value any) any {
	return normalize(value, nil)
}

// NormalizeWithIgnore is like Normalize but also strips user-configured field names.
func NormalizeWithIgnore(value any, ignoreFields []string) any {
	extra := make(map[string]bool, len(ignoreFields))
	for _, f := range ignoreFields {
		extra[f] = true
	}
	return normalize(value, extra)
}

func normalize(value any, extra map[string]bool) any {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		if reISO8601.MatchString(v) {
			return "date"
		}
		if reUUID.MatchString(v) {
			return "uuid"
		}
		if reJWT.MatchString(v) {
			return "token"
		}
		return "string"
	case float64:
		return "number"
	case int, int64:
		return "number"
	case bool:
		return "boolean"
	case []any:
		if len(v) == 0 {
			return "empty_array"
		}
		return []any{normalize(v[0], extra)}
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, nested := range v {
			if dynamicKeys[key] {
				continue
			}
			if extra != nil && extra[key] {
				continue
			}
			out[key] = normalize(nested, extra)
		}
		return out
	default:
		return fmt.Sprintf("%T", value)
	}
}

// HashSchema returns a deterministic SHA-256 hex string for a normalized value.
// The value is serialized with sorted keys before hashing.
func HashSchema(normalized any) string {
	serialized := serializeSorted(normalized)
	sum := sha256.Sum256([]byte(serialized))
	return fmt.Sprintf("%x", sum)
}

// serializeSorted produces a deterministic string from a normalized value by
// sorting map keys before serialization.
func serializeSorted(v any) string {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+":"+serializeSorted(val[k]))
		}
		return "{" + strings.Join(parts, ",") + "}"
	case []any:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			parts = append(parts, serializeSorted(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// NormalizeAndHash parses raw JSON bytes, normalizes the result, and returns
// the schema hash. Returns an empty string if the body is not valid JSON.
func NormalizeAndHash(body []byte, ignoreFields []string) string {
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	normalized := NormalizeWithIgnore(parsed, ignoreFields)
	return HashSchema(normalized)
}

// NormalizeAndHashWithShape is like NormalizeAndHash but also returns the
// normalized shape as JSON bytes. Used by the snapshot engine to store the
// shape for field-level diff in rg check.
// Returns empty string and nil if the body is not valid JSON.
func NormalizeAndHashWithShape(body []byte, ignoreFields []string) (hash string, shapeJSON []byte) {
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", nil
	}
	normalized := NormalizeWithIgnore(parsed, ignoreFields)
	h := HashSchema(normalized)
	b, err := json.Marshal(normalized)
	if err != nil {
		return h, nil
	}
	return h, b
}
