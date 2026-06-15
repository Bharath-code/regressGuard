// Package snapshot defines the Snapshot type and handles reading/writing
// .regressguard/snapshot.json.
package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DirName  = ".regressguard"
	FileName = "snapshot.json"
	Version  = 1
)

// Snapshot is the persisted known-good baseline.
type Snapshot struct {
	Version   int                    `json:"version"`
	CreatedAt time.Time              `json:"createdAt"`
	GitCommit string                 `json:"gitCommit"`
	Tests     TestSummary            `json:"tests"`
	Routes    map[string]RouteRecord `json:"routes"`
}

// TestSummary records the test suite outcome at snapshot time.
type TestSummary struct {
	Passed   int           `json:"passed"`
	Failed   int           `json:"failed"`
	Skipped  int           `json:"skipped"`
	Duration time.Duration `json:"durationMs"`
}

// RouteRecord holds the captured state for a single route.
type RouteRecord struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	SchemaHash string `json:"schemaHash"`
	// NormalizedSchema stores the type-shape of the response for field-level diff.
	// It is the output of engine.NormalizeWithIgnore serialized as JSON.
	// Populated from snapshot version 1 onward.
	NormalizedSchema json.RawMessage `json:"normalizedSchema,omitempty"`
	MS               int64           `json:"ms"`
	// Unverified marks a route that could not be measured during a check run
	// (transient timeout, connection error). Such routes are reported as a
	// non-blocking WARNING by the diff engine — never a CRITICAL regression —
	// so a network blip cannot block a commit. Only set on the "after" side of
	// a check; never persisted into a baseline snapshot.
	Unverified       bool   `json:"unverified,omitempty"`
	UnverifiedReason string `json:"unverifiedReason,omitempty"`
}

// RouteKey returns the canonical map key for a route.
func RouteKey(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

// Path returns the absolute path to snapshot.json given a project root.
func Path(root string) string {
	return filepath.Join(root, DirName, FileName)
}

// Exists reports whether a snapshot file is present.
func Exists(root string) bool {
	_, err := os.Stat(Path(root))
	return err == nil
}

// Write serializes and saves the snapshot to disk.
// If redactFields is non-empty, those field names are stripped from
// NormalizedSchema before persisting (S2: snapshot sanitization).
func Write(root string, snap Snapshot, redactFields ...[]string) error {
	dir := filepath.Join(root, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	// S2: apply redaction to NormalizedSchema if redactFields provided.
	if len(redactFields) > 0 && len(redactFields[0]) > 0 {
		snap = redactSnapshot(snap, redactFields[0])
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(Path(root), data, 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}

	// S7: write HMAC for integrity verification.
	if err := WriteHMAC(root); err != nil {
		// Non-fatal — integrity is a bonus, not a requirement.
		_ = err
	}

	return nil
}

// redactSnapshot returns a copy of the snapshot with specified field names
// removed from all NormalizedSchema entries. This prevents accidental exposure
// of internal field names in committed snapshots.
func redactSnapshot(snap Snapshot, fields []string) Snapshot {
	if len(fields) == 0 {
		return snap
	}
	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[f] = true
	}

	// Deep copy routes map.
	newRoutes := make(map[string]RouteRecord, len(snap.Routes))
	for key, record := range snap.Routes {
		if len(record.NormalizedSchema) > 0 {
			record.NormalizedSchema = redactJSON(record.NormalizedSchema, fieldSet)
		}
		newRoutes[key] = record
	}
	snap.Routes = newRoutes
	return snap
}

// redactJSON removes specified keys from a JSON object (top-level and nested).
func redactJSON(data json.RawMessage, fields map[string]bool) json.RawMessage {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return data // not an object, return as-is
	}

	for key := range fields {
		delete(obj, key)
	}

	// Recurse into nested objects.
	for key, val := range obj {
		obj[key] = redactJSON(val, fields)
	}

	result, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	return result
}

// Load reads and parses the snapshot from disk.
func Load(root string) (Snapshot, error) {
	data, err := os.ReadFile(Path(root))
	if err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot: %w", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("parse snapshot: %w", err)
	}
	return snap, nil
}

// GitCommit returns the short HEAD commit hash, or "unknown" if git is
// unavailable or the directory is not a git repo.
func GitCommit(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
