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
func Write(root string, snap Snapshot) error {
	dir := filepath.Join(root, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(Path(root), data, 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
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
