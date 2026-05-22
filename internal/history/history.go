// Package history manages the .regressguard/history/ directory and the
// index.json file that tracks archived snapshots.
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Bharath-code/regressguard/internal/failures"
	"github.com/Bharath-code/regressguard/internal/snapshot"
)

const (
	DirName       = "history"
	IndexFileName = "index.json"
	DefaultMax    = 20
)

// TestMeta records the test suite outcome summary for an archived snapshot.
type TestMeta struct {
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

// IndexEntry represents one archived snapshot in the history index.
type IndexEntry struct {
	File      string   `json:"file"`      // relative path: "20260520-133138-2b25619.json"
	GitCommit string   `json:"gitCommit"` // short hash or "unknown"
	CreatedAt time.Time `json:"createdAt"` // RFC 3339
	Tests     TestMeta `json:"tests"`     // summary for listing
}

// Index is the full history index file.
type Index struct {
	Entries []IndexEntry `json:"entries"` // ordered newest-first
}

// HistoryDir returns the absolute path to .regressguard/history/ given a project root.
func HistoryDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".regressguard", DirName)
}

// IndexPath returns the absolute path to .regressguard/history/index.json.
func IndexPath(projectRoot string) string {
	return filepath.Join(HistoryDir(projectRoot), IndexFileName)
}

// LoadIndex reads and parses .regressguard/history/index.json.
// Returns an empty Index if the file does not exist.
func LoadIndex(projectRoot string) (Index, error) {
	data, err := os.ReadFile(IndexPath(projectRoot))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Index{}, nil
		}
		return Index{}, fmt.Errorf("read history index: %w", err)
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return Index{}, fmt.Errorf("parse history index: %w", err)
	}
	return idx, nil
}

// SaveIndex writes the index to .regressguard/history/index.json with indented formatting.
func SaveIndex(projectRoot string, idx Index) error {
	dir := HistoryDir(projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal history index: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(IndexPath(projectRoot), data, 0o644); err != nil {
		return fmt.Errorf("write history index: %w", err)
	}
	return nil
}

// ArchiveFileName generates the archive filename from a snapshot's metadata.
// Format: YYYYMMDD-HHmmss-<gitcommit>.json
func ArchiveFileName(createdAt time.Time, gitCommit string) string {
	ts := createdAt.UTC().Format("20060102-150405")
	return fmt.Sprintf("%s-%s.json", ts, gitCommit)
}

// Archive copies the given snapshot to .regressguard/history/ and updates the index.
func Archive(projectRoot string, snap snapshot.Snapshot) error {
	dir := HistoryDir(projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}

	// Generate archive filename.
	filename := ArchiveFileName(snap.CreatedAt, snap.GitCommit)

	// Marshal the snapshot to JSON.
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot for archive: %w", err)
	}
	data = append(data, '\n')

	// Write the snapshot file to the history directory.
	archivePath := filepath.Join(dir, filename)
	if err := os.WriteFile(archivePath, data, 0o644); err != nil {
		return fmt.Errorf("write archive file: %w", err)
	}

	// Load existing index (or create empty).
	idx, err := LoadIndex(projectRoot)
	if err != nil {
		return fmt.Errorf("load index for archive: %w", err)
	}

	// Prepend new entry to the index (newest-first ordering).
	entry := IndexEntry{
		File:      filename,
		GitCommit: snap.GitCommit,
		CreatedAt: snap.CreatedAt,
		Tests: TestMeta{
			Passed: snap.Tests.Passed,
			Failed: snap.Tests.Failed,
		},
	}
	idx.Entries = append([]IndexEntry{entry}, idx.Entries...)

	// Save the updated index.
	if err := SaveIndex(projectRoot, idx); err != nil {
		return fmt.Errorf("save index after archive: %w", err)
	}

	return nil
}

// Prune removes entries beyond maxHistory from the index, deleting their files
// from disk. If a file is already gone, a warning is logged but pruning continues.
// The updated index is saved after all removals.
func Prune(projectRoot string, idx *Index, maxHistory int) {
	dir := HistoryDir(projectRoot)

	for len(idx.Entries) > maxHistory {
		// Remove the last entry (oldest, since entries are newest-first).
		last := idx.Entries[len(idx.Entries)-1]
		idx.Entries = idx.Entries[:len(idx.Entries)-1]

		// Delete the corresponding file from disk.
		filePath := filepath.Join(dir, last.File)
		if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("warning: failed to delete history file %s: %v", last.File, err)
		} else if errors.Is(err, os.ErrNotExist) {
			log.Printf("warning: history file already deleted: %s", last.File)
		}
	}

	// Save the updated index after pruning.
	if err := SaveIndex(projectRoot, *idx); err != nil {
		log.Printf("warning: failed to save index after pruning: %v", err)
	}
}

// ListEntry represents a single snapshot entry for display in the list view.
type ListEntry struct {
	Ref        string
	GitCommit  string
	CreatedAt  time.Time
	TestPassed int
}

// List returns all available snapshots (latest + history) for display.
// The current snapshot.json is included as "latest" at the top, followed by
// historical entries mapped to ~1, ~2, etc.
// Returns failures.Actionable if no snapshot exists at all.
func List(projectRoot string) ([]ListEntry, error) {
	// Load the current snapshot.json.
	snap, err := snapshot.Load(projectRoot)
	if err != nil {
		return nil, failures.Actionable{
			Title:       "No snapshot available",
			Cause:       "No snapshot.json found — nothing to list",
			Next:        "rg snapshot",
			MoreContext: "rg diff --help",
		}
	}

	// Start with the "latest" entry.
	entries := []ListEntry{
		{
			Ref:        "latest",
			GitCommit:  snap.GitCommit,
			CreatedAt:  snap.CreatedAt,
			TestPassed: snap.Tests.Passed,
		},
	}

	// Load the history index and append entries.
	idx, err := LoadIndex(projectRoot)
	if err != nil {
		// If we can't load the index, just return the latest entry.
		return entries, nil
	}

	for i, entry := range idx.Entries {
		entries = append(entries, ListEntry{
			Ref:        fmt.Sprintf("~%d", i+1),
			GitCommit:  entry.GitCommit,
			CreatedAt:  entry.CreatedAt,
			TestPassed: entry.Tests.Passed,
		})
	}

	return entries, nil
}

// ResolvedMeta contains metadata about the resolved snapshot for display.
type ResolvedMeta struct {
	GitCommit string
	CreatedAt time.Time
	Ref       string
}

// tildeRegex matches ~N references (e.g., ~1, ~2, ~10).
var tildeRegex = regexp.MustCompile(`^~(\d+)$`)

// commitRegex matches git commit short hashes (4-40 hex characters).
var commitRegex = regexp.MustCompile(`^[0-9a-fA-F]{4,40}$`)

// Resolve resolves a Snapshot_Ref string to a loaded Snapshot and its metadata.
// Supported refs:
//   - "latest": loads snapshot.json from the project root
//   - "~N": loads the Nth most recent entry from the index (1-indexed)
//   - hex string (4-40 chars): case-insensitive prefix match on git commit
func Resolve(projectRoot string, ref string, idx Index) (snapshot.Snapshot, ResolvedMeta, error) {
	switch {
	case ref == "latest":
		return resolveLatest(projectRoot)
	case tildeRegex.MatchString(ref):
		return resolveTilde(projectRoot, ref, idx)
	case commitRegex.MatchString(ref):
		return resolveCommit(projectRoot, ref, idx)
	default:
		return snapshot.Snapshot{}, ResolvedMeta{}, failures.Actionable{
			Title:       "Snapshot not found",
			Cause:       fmt.Sprintf("Reference %q does not match any known format (latest, ~N, or git commit hash)", ref),
			Next:        "rg diff --list",
			MoreContext: "rg diff --help",
		}
	}
}

// resolveLatest loads the current snapshot.json from the project root.
func resolveLatest(projectRoot string) (snapshot.Snapshot, ResolvedMeta, error) {
	snap, err := snapshot.Load(projectRoot)
	if err != nil {
		return snapshot.Snapshot{}, ResolvedMeta{}, failures.Actionable{
			Title:       "Snapshot file missing",
			Cause:       fmt.Sprintf("Could not load snapshot.json: %v", err),
			Next:        "rg snapshot",
			MoreContext: "rg diff --help",
		}
	}
	meta := ResolvedMeta{
		GitCommit: snap.GitCommit,
		CreatedAt: snap.CreatedAt,
		Ref:       "latest",
	}
	return snap, meta, nil
}

// resolveTilde resolves a ~N reference to the Nth most recent index entry.
func resolveTilde(projectRoot string, ref string, idx Index) (snapshot.Snapshot, ResolvedMeta, error) {
	matches := tildeRegex.FindStringSubmatch(ref)
	n, _ := strconv.Atoi(matches[1])

	if n < 1 {
		return snapshot.Snapshot{}, ResolvedMeta{}, failures.Actionable{
			Title:       "Snapshot not found",
			Cause:       fmt.Sprintf("~N requires N >= 1, got ~%d", n),
			Next:        "rg diff --list",
			MoreContext: "rg diff --help",
		}
	}

	if n > len(idx.Entries) {
		return snapshot.Snapshot{}, ResolvedMeta{}, failures.Actionable{
			Title:       "Snapshot not found",
			Cause:       fmt.Sprintf("Only %d historical snapshots available, but ~%d was requested", len(idx.Entries), n),
			Next:        "rg diff --list",
			MoreContext: "rg diff --help",
		}
	}

	entry := idx.Entries[n-1]
	snap, err := loadArchiveFile(projectRoot, entry.File)
	if err != nil {
		return snapshot.Snapshot{}, ResolvedMeta{}, err
	}

	meta := ResolvedMeta{
		GitCommit: entry.GitCommit,
		CreatedAt: entry.CreatedAt,
		Ref:       ref,
	}
	return snap, meta, nil
}

// resolveCommit resolves a git commit prefix to a matching index entry.
func resolveCommit(projectRoot string, ref string, idx Index) (snapshot.Snapshot, ResolvedMeta, error) {
	lowerRef := strings.ToLower(ref)
	var matches []IndexEntry

	for _, entry := range idx.Entries {
		if strings.HasPrefix(strings.ToLower(entry.GitCommit), lowerRef) {
			matches = append(matches, entry)
		}
	}

	if len(matches) == 0 {
		return snapshot.Snapshot{}, ResolvedMeta{}, failures.Actionable{
			Title:       "Snapshot not found",
			Cause:       fmt.Sprintf("No snapshot matches commit prefix %q", ref),
			Next:        "rg diff --list",
			MoreContext: "rg diff --help",
		}
	}

	if len(matches) > 1 {
		// List up to 5 matching commits.
		limit := len(matches)
		if limit > 5 {
			limit = 5
		}
		var commitList []string
		for i := 0; i < limit; i++ {
			commitList = append(commitList, matches[i].GitCommit)
		}
		cause := fmt.Sprintf("Prefix %q matches %d snapshots: %s", ref, len(matches), strings.Join(commitList, ", "))
		if len(matches) > 5 {
			cause += fmt.Sprintf(" (and %d more)", len(matches)-5)
		}
		return snapshot.Snapshot{}, ResolvedMeta{}, failures.Actionable{
			Title:       "Ambiguous snapshot reference",
			Cause:       cause,
			Next:        "Use a longer commit prefix to narrow the match",
			MoreContext: "rg diff --list",
		}
	}

	entry := matches[0]
	snap, err := loadArchiveFile(projectRoot, entry.File)
	if err != nil {
		return snapshot.Snapshot{}, ResolvedMeta{}, err
	}

	meta := ResolvedMeta{
		GitCommit: entry.GitCommit,
		CreatedAt: entry.CreatedAt,
		Ref:       ref,
	}
	return snap, meta, nil
}

// loadArchiveFile loads a snapshot from the history directory by filename.
func loadArchiveFile(projectRoot string, filename string) (snapshot.Snapshot, error) {
	filePath := filepath.Join(HistoryDir(projectRoot), filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return snapshot.Snapshot{}, failures.Actionable{
			Title:       "Snapshot file missing",
			Cause:       fmt.Sprintf("Referenced file %q is missing from disk", filename),
			Next:        "rg snapshot",
			MoreContext: "rg diff --help",
		}
	}

	var snap snapshot.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return snapshot.Snapshot{}, fmt.Errorf("parse archive file %s: %w", filename, err)
	}
	return snap, nil
}
