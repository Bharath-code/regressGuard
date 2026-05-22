package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Bharath-code/regressguard/internal/failures"
	"github.com/Bharath-code/regressguard/internal/snapshot"
)

func TestLoadIndex_nonExistentReturnsEmpty(t *testing.T) {
	dir := t.TempDir()

	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(idx.Entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(idx.Entries))
	}
}

func TestSaveAndLoadIndex_roundTrip(t *testing.T) {
	dir := t.TempDir()

	now := time.Now().UTC().Truncate(time.Second)
	original := Index{
		Entries: []IndexEntry{
			{
				File:      "20260520-133138-2b25619.json",
				GitCommit: "2b25619",
				CreatedAt: now,
				Tests:     TestMeta{Passed: 4, Failed: 0},
			},
			{
				File:      "20260519-091022-a1f8c02.json",
				GitCommit: "a1f8c02",
				CreatedAt: now.Add(-24 * time.Hour),
				Tests:     TestMeta{Passed: 3, Failed: 1},
			},
		},
	}

	if err := SaveIndex(dir, original); err != nil {
		t.Fatalf("SaveIndex failed: %v", err)
	}

	// Verify the file was created.
	if _, err := os.Stat(IndexPath(dir)); err != nil {
		t.Fatalf("index file not created: %v", err)
	}

	// Load it back.
	loaded, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	if len(loaded.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded.Entries))
	}

	// Verify first entry.
	e := loaded.Entries[0]
	if e.File != "20260520-133138-2b25619.json" {
		t.Errorf("entry[0].File = %q, want %q", e.File, "20260520-133138-2b25619.json")
	}
	if e.GitCommit != "2b25619" {
		t.Errorf("entry[0].GitCommit = %q, want %q", e.GitCommit, "2b25619")
	}
	if !e.CreatedAt.Equal(now) {
		t.Errorf("entry[0].CreatedAt = %v, want %v", e.CreatedAt, now)
	}
	if e.Tests.Passed != 4 || e.Tests.Failed != 0 {
		t.Errorf("entry[0].Tests = %+v, want {Passed:4 Failed:0}", e.Tests)
	}

	// Verify second entry.
	e2 := loaded.Entries[1]
	if e2.File != "20260519-091022-a1f8c02.json" {
		t.Errorf("entry[1].File = %q, want %q", e2.File, "20260519-091022-a1f8c02.json")
	}
	if e2.Tests.Passed != 3 || e2.Tests.Failed != 1 {
		t.Errorf("entry[1].Tests = %+v, want {Passed:3 Failed:1}", e2.Tests)
	}
}

func TestLoadIndex_corruptedReturnsError(t *testing.T) {
	dir := t.TempDir()

	// Create the history directory and write invalid JSON.
	histDir := filepath.Join(dir, ".regressguard", "history")
	if err := os.MkdirAll(histDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(histDir, "index.json"), []byte("{invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadIndex(dir)
	if err == nil {
		t.Fatal("expected error for corrupted index, got nil")
	}
}

func makeTestSnapshot(t *testing.T, createdAt time.Time, gitCommit string) snapshot.Snapshot {
	t.Helper()
	return snapshot.Snapshot{
		Version:   1,
		CreatedAt: createdAt,
		GitCommit: gitCommit,
		Tests: snapshot.TestSummary{
			Passed:  4,
			Failed:  1,
			Skipped: 0,
		},
		Routes: map[string]snapshot.RouteRecord{
			"GET /api/health": {
				Method: "GET",
				Path:   "/api/health",
				Status: 200,
				MS:     42,
			},
		},
	}
}

func TestArchive_createsFileAndUpdatesIndex(t *testing.T) {
	dir := t.TempDir()

	now := time.Date(2026, 5, 20, 13, 31, 38, 0, time.UTC)
	snap := makeTestSnapshot(t, now, "2b25619")

	if err := Archive(dir, snap); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	// Verify the archive file was created.
	expectedFile := "20260520-133138-2b25619.json"
	archivePath := filepath.Join(HistoryDir(dir), expectedFile)
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("archive file not created: %v", err)
	}

	// Verify the file contains valid snapshot JSON.
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive file: %v", err)
	}
	var loaded snapshot.Snapshot
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal archive file: %v", err)
	}
	if loaded.GitCommit != "2b25619" {
		t.Errorf("archived snapshot GitCommit = %q, want %q", loaded.GitCommit, "2b25619")
	}
	if loaded.Tests.Passed != 4 {
		t.Errorf("archived snapshot Tests.Passed = %d, want 4", loaded.Tests.Passed)
	}

	// Verify the index was updated.
	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}
	if len(idx.Entries) != 1 {
		t.Fatalf("expected 1 index entry, got %d", len(idx.Entries))
	}
	entry := idx.Entries[0]
	if entry.File != expectedFile {
		t.Errorf("index entry File = %q, want %q", entry.File, expectedFile)
	}
	if entry.GitCommit != "2b25619" {
		t.Errorf("index entry GitCommit = %q, want %q", entry.GitCommit, "2b25619")
	}
	if !entry.CreatedAt.Equal(now) {
		t.Errorf("index entry CreatedAt = %v, want %v", entry.CreatedAt, now)
	}
	if entry.Tests.Passed != 4 || entry.Tests.Failed != 1 {
		t.Errorf("index entry Tests = %+v, want {Passed:4 Failed:1}", entry.Tests)
	}
}

func TestArchive_filenameFormat(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name      string
		createdAt time.Time
		commit    string
		wantFile  string
	}{
		{
			name:      "standard commit",
			createdAt: time.Date(2026, 5, 20, 13, 31, 38, 0, time.UTC),
			commit:    "2b25619",
			wantFile:  "20260520-133138-2b25619.json",
		},
		{
			name:      "unknown commit",
			createdAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			commit:    "unknown",
			wantFile:  "20260101-000000-unknown.json",
		},
		{
			name:      "midnight edge case",
			createdAt: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
			commit:    "abc1234",
			wantFile:  "20251231-235959-abc1234.json",
		},
	}

	// Verify the filename format matches YYYYMMDD-HHmmss-<commit>.json
	pattern := regexp.MustCompile(`^\d{8}-\d{6}-.+\.json$`)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filename := ArchiveFileName(tc.createdAt, tc.commit)
			if filename != tc.wantFile {
				t.Errorf("ArchiveFileName() = %q, want %q", filename, tc.wantFile)
			}
			if !pattern.MatchString(filename) {
				t.Errorf("filename %q does not match expected pattern YYYYMMDD-HHmmss-<commit>.json", filename)
			}
		})
	}

	// Also verify that Archive actually uses this filename.
	snap := makeTestSnapshot(t, time.Date(2026, 3, 15, 10, 20, 30, 0, time.UTC), "f3e9b11")
	if err := Archive(dir, snap); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}
	expectedPath := filepath.Join(HistoryDir(dir), "20260315-102030-f3e9b11.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected archive file at %s, got error: %v", expectedPath, err)
	}
}

func TestArchive_createsDirectoryIfMissing(t *testing.T) {
	dir := t.TempDir()

	// Verify the history directory does not exist yet.
	histDir := HistoryDir(dir)
	if _, err := os.Stat(histDir); err == nil {
		t.Fatal("history directory should not exist before Archive")
	}

	snap := makeTestSnapshot(t, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC), "deadbee")

	if err := Archive(dir, snap); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	// Verify the directory was created.
	info, err := os.Stat(histDir)
	if err != nil {
		t.Fatalf("history directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("history path is not a directory")
	}

	// Verify the file exists inside it.
	expectedFile := "20260601-120000-deadbee.json"
	archivePath := filepath.Join(histDir, expectedFile)
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("archive file not created in new directory: %v", err)
	}

	// Verify index was also created.
	indexPath := IndexPath(dir)
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("index file not created: %v", err)
	}
}

func TestPrune_removesOldestEntries(t *testing.T) {
	dir := t.TempDir()

	// Create 5 archived snapshots.
	baseTime := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		snap := makeTestSnapshot(t, baseTime.Add(time.Duration(i)*time.Hour), fmt.Sprintf("commit%d", i))
		if err := Archive(dir, snap); err != nil {
			t.Fatalf("Archive %d failed: %v", i, err)
		}
	}

	// Verify we have 5 entries.
	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}
	if len(idx.Entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(idx.Entries))
	}

	// Prune to max 3.
	Prune(dir, &idx, 3)

	// Verify only 3 entries remain.
	if len(idx.Entries) != 3 {
		t.Fatalf("expected 3 entries after prune, got %d", len(idx.Entries))
	}

	// Verify the remaining entries are the newest (commit4, commit3, commit2).
	// Index is newest-first, so commit4 is first.
	if idx.Entries[0].GitCommit != "commit4" {
		t.Errorf("entry[0].GitCommit = %q, want %q", idx.Entries[0].GitCommit, "commit4")
	}
	if idx.Entries[1].GitCommit != "commit3" {
		t.Errorf("entry[1].GitCommit = %q, want %q", idx.Entries[1].GitCommit, "commit3")
	}
	if idx.Entries[2].GitCommit != "commit2" {
		t.Errorf("entry[2].GitCommit = %q, want %q", idx.Entries[2].GitCommit, "commit2")
	}

	// Verify the pruned files (commit0, commit1) are deleted from disk.
	for i := 0; i < 2; i++ {
		filename := ArchiveFileName(baseTime.Add(time.Duration(i)*time.Hour), fmt.Sprintf("commit%d", i))
		path := filepath.Join(HistoryDir(dir), filename)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("expected file %s to be deleted, but it still exists", filename)
		}
	}

	// Verify the kept files (commit2, commit3, commit4) still exist.
	for i := 2; i < 5; i++ {
		filename := ArchiveFileName(baseTime.Add(time.Duration(i)*time.Hour), fmt.Sprintf("commit%d", i))
		path := filepath.Join(HistoryDir(dir), filename)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to still exist, got error: %v", filename, err)
		}
	}

	// Verify the saved index on disk matches.
	reloaded, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex after prune failed: %v", err)
	}
	if len(reloaded.Entries) != 3 {
		t.Errorf("expected 3 entries in saved index, got %d", len(reloaded.Entries))
	}
}

func TestPrune_noOpWhenUnderLimit(t *testing.T) {
	dir := t.TempDir()

	// Create 3 archived snapshots.
	baseTime := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		snap := makeTestSnapshot(t, baseTime.Add(time.Duration(i)*time.Hour), fmt.Sprintf("abc%04d", i))
		if err := Archive(dir, snap); err != nil {
			t.Fatalf("Archive %d failed: %v", i, err)
		}
	}

	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	// Prune with maxHistory=5 (above current count of 3).
	Prune(dir, &idx, 5)

	// Verify all 3 entries remain.
	if len(idx.Entries) != 3 {
		t.Errorf("expected 3 entries (no pruning), got %d", len(idx.Entries))
	}

	// Verify all files still exist on disk.
	for i := 0; i < 3; i++ {
		filename := ArchiveFileName(baseTime.Add(time.Duration(i)*time.Hour), fmt.Sprintf("abc%04d", i))
		path := filepath.Join(HistoryDir(dir), filename)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to still exist, got error: %v", filename, err)
		}
	}
}

func TestPrune_handlesAlreadyDeletedFiles(t *testing.T) {
	dir := t.TempDir()

	// Create 4 archived snapshots.
	baseTime := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		snap := makeTestSnapshot(t, baseTime.Add(time.Duration(i)*time.Hour), fmt.Sprintf("del%04d", i))
		if err := Archive(dir, snap); err != nil {
			t.Fatalf("Archive %d failed: %v", i, err)
		}
	}

	// Manually delete the oldest file (del0000) before pruning.
	oldestFile := ArchiveFileName(baseTime, "del0000")
	oldestPath := filepath.Join(HistoryDir(dir), oldestFile)
	if err := os.Remove(oldestPath); err != nil {
		t.Fatalf("failed to pre-delete file: %v", err)
	}

	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	// Prune to max 2 — this should remove del0000 (already gone) and del0001.
	// It should NOT panic or fail.
	Prune(dir, &idx, 2)

	// Verify only 2 entries remain.
	if len(idx.Entries) != 2 {
		t.Fatalf("expected 2 entries after prune, got %d", len(idx.Entries))
	}

	// Verify the remaining entries are the newest.
	if idx.Entries[0].GitCommit != "del0003" {
		t.Errorf("entry[0].GitCommit = %q, want %q", idx.Entries[0].GitCommit, "del0003")
	}
	if idx.Entries[1].GitCommit != "del0002" {
		t.Errorf("entry[1].GitCommit = %q, want %q", idx.Entries[1].GitCommit, "del0002")
	}

	// Verify del0001 file was also deleted.
	del1File := ArchiveFileName(baseTime.Add(time.Hour), "del0001")
	del1Path := filepath.Join(HistoryDir(dir), del1File)
	if _, err := os.Stat(del1Path); err == nil {
		t.Error("expected del0001 file to be deleted, but it still exists")
	}
}

// --- Resolve tests ---

func TestResolve_latest(t *testing.T) {
	dir := t.TempDir()

	// Create a snapshot.json in the project root.
	now := time.Date(2026, 5, 20, 13, 31, 38, 0, time.UTC)
	snap := makeTestSnapshot(t, now, "abc1234")
	if err := snapshot.Write(dir, snap); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}

	idx := Index{} // empty index is fine for "latest"

	resolved, meta, err := Resolve(dir, "latest", idx)
	if err != nil {
		t.Fatalf("Resolve(latest) returned error: %v", err)
	}

	if meta.Ref != "latest" {
		t.Errorf("meta.Ref = %q, want %q", meta.Ref, "latest")
	}
	if meta.GitCommit != "abc1234" {
		t.Errorf("meta.GitCommit = %q, want %q", meta.GitCommit, "abc1234")
	}
	if !meta.CreatedAt.Equal(now) {
		t.Errorf("meta.CreatedAt = %v, want %v", meta.CreatedAt, now)
	}
	if resolved.GitCommit != "abc1234" {
		t.Errorf("resolved.GitCommit = %q, want %q", resolved.GitCommit, "abc1234")
	}
	if resolved.Tests.Passed != 4 {
		t.Errorf("resolved.Tests.Passed = %d, want 4", resolved.Tests.Passed)
	}
}

func TestResolve_latest_missingFile(t *testing.T) {
	dir := t.TempDir()

	idx := Index{}

	_, _, err := Resolve(dir, "latest", idx)
	if err == nil {
		t.Fatal("expected error for missing snapshot.json, got nil")
	}

	actionable, ok := err.(failures.Actionable)
	if !ok {
		t.Fatalf("expected failures.Actionable, got %T: %v", err, err)
	}
	if actionable.Title != "Snapshot file missing" {
		t.Errorf("error title = %q, want %q", actionable.Title, "Snapshot file missing")
	}
}

func TestResolve_tildeN(t *testing.T) {
	dir := t.TempDir()

	// Create 3 archived snapshots.
	baseTime := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		snap := makeTestSnapshot(t, baseTime.Add(time.Duration(i)*time.Hour), fmt.Sprintf("commit%d", i))
		if err := Archive(dir, snap); err != nil {
			t.Fatalf("Archive %d failed: %v", i, err)
		}
	}

	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	// ~1 should be the most recent (commit2, since index is newest-first).
	resolved, meta, err := Resolve(dir, "~1", idx)
	if err != nil {
		t.Fatalf("Resolve(~1) returned error: %v", err)
	}
	if meta.GitCommit != "commit2" {
		t.Errorf("~1 meta.GitCommit = %q, want %q", meta.GitCommit, "commit2")
	}
	if meta.Ref != "~1" {
		t.Errorf("~1 meta.Ref = %q, want %q", meta.Ref, "~1")
	}
	if resolved.GitCommit != "commit2" {
		t.Errorf("~1 resolved.GitCommit = %q, want %q", resolved.GitCommit, "commit2")
	}

	// ~3 should be the oldest (commit0).
	resolved, meta, err = Resolve(dir, "~3", idx)
	if err != nil {
		t.Fatalf("Resolve(~3) returned error: %v", err)
	}
	if meta.GitCommit != "commit0" {
		t.Errorf("~3 meta.GitCommit = %q, want %q", meta.GitCommit, "commit0")
	}
	if meta.Ref != "~3" {
		t.Errorf("~3 meta.Ref = %q, want %q", meta.Ref, "~3")
	}
	if resolved.GitCommit != "commit0" {
		t.Errorf("~3 resolved.GitCommit = %q, want %q", resolved.GitCommit, "commit0")
	}
}

func TestResolve_gitCommit(t *testing.T) {
	dir := t.TempDir()

	// Create archived snapshots with distinct commit hashes.
	baseTime := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	commits := []string{"abc1234", "def5678", "1a2b3c4"}
	for i, commit := range commits {
		snap := makeTestSnapshot(t, baseTime.Add(time.Duration(i)*time.Hour), commit)
		if err := Archive(dir, snap); err != nil {
			t.Fatalf("Archive %d failed: %v", i, err)
		}
	}

	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	// Full match.
	resolved, meta, err := Resolve(dir, "abc1234", idx)
	if err != nil {
		t.Fatalf("Resolve(abc1234) returned error: %v", err)
	}
	if meta.GitCommit != "abc1234" {
		t.Errorf("meta.GitCommit = %q, want %q", meta.GitCommit, "abc1234")
	}
	if resolved.GitCommit != "abc1234" {
		t.Errorf("resolved.GitCommit = %q, want %q", resolved.GitCommit, "abc1234")
	}

	// Prefix match (case-insensitive).
	resolved, meta, err = Resolve(dir, "DEF5", idx)
	if err != nil {
		t.Fatalf("Resolve(DEF5) returned error: %v", err)
	}
	if meta.GitCommit != "def5678" {
		t.Errorf("meta.GitCommit = %q, want %q", meta.GitCommit, "def5678")
	}

	// Prefix match with lowercase.
	resolved, meta, err = Resolve(dir, "1a2b", idx)
	if err != nil {
		t.Fatalf("Resolve(1a2b) returned error: %v", err)
	}
	if meta.GitCommit != "1a2b3c4" {
		t.Errorf("meta.GitCommit = %q, want %q", meta.GitCommit, "1a2b3c4")
	}
}

func TestResolve_gitCommit_notFound(t *testing.T) {
	dir := t.TempDir()

	baseTime := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	snap := makeTestSnapshot(t, baseTime, "abc1234")
	if err := Archive(dir, snap); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	// Try a commit that doesn't match anything.
	_, _, err = Resolve(dir, "ffff", idx)
	if err == nil {
		t.Fatal("expected error for non-matching commit, got nil")
	}

	actionable, ok := err.(failures.Actionable)
	if !ok {
		t.Fatalf("expected failures.Actionable, got %T: %v", err, err)
	}
	if actionable.Title != "Snapshot not found" {
		t.Errorf("error title = %q, want %q", actionable.Title, "Snapshot not found")
	}
}

func TestResolve_outOfRange(t *testing.T) {
	dir := t.TempDir()

	// Create 2 archived snapshots.
	baseTime := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		snap := makeTestSnapshot(t, baseTime.Add(time.Duration(i)*time.Hour), fmt.Sprintf("commit%d", i))
		if err := Archive(dir, snap); err != nil {
			t.Fatalf("Archive %d failed: %v", i, err)
		}
	}

	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	// ~3 is out of range (only 2 entries).
	_, _, err = Resolve(dir, "~3", idx)
	if err == nil {
		t.Fatal("expected error for ~3 out of range, got nil")
	}

	actionable, ok := err.(failures.Actionable)
	if !ok {
		t.Fatalf("expected failures.Actionable, got %T: %v", err, err)
	}
	if actionable.Title != "Snapshot not found" {
		t.Errorf("error title = %q, want %q", actionable.Title, "Snapshot not found")
	}
	expectedCause := "Only 2 historical snapshots available, but ~3 was requested"
	if actionable.Cause != expectedCause {
		t.Errorf("error cause = %q, want %q", actionable.Cause, expectedCause)
	}
	if actionable.Next != "rg diff --list" {
		t.Errorf("error next = %q, want %q", actionable.Next, "rg diff --list")
	}
}

func TestResolve_ambiguousCommit(t *testing.T) {
	dir := t.TempDir()

	// Create snapshots with commits that share a prefix.
	baseTime := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	commits := []string{"abcd111", "abcd222", "abcd333"}
	for i, commit := range commits {
		snap := makeTestSnapshot(t, baseTime.Add(time.Duration(i)*time.Hour), commit)
		if err := Archive(dir, snap); err != nil {
			t.Fatalf("Archive %d failed: %v", i, err)
		}
	}

	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	// "abcd" matches all 3 entries.
	_, _, err = Resolve(dir, "abcd", idx)
	if err == nil {
		t.Fatal("expected error for ambiguous commit, got nil")
	}

	actionable, ok := err.(failures.Actionable)
	if !ok {
		t.Fatalf("expected failures.Actionable, got %T: %v", err, err)
	}
	if actionable.Title != "Ambiguous snapshot reference" {
		t.Errorf("error title = %q, want %q", actionable.Title, "Ambiguous snapshot reference")
	}
	// Should mention the prefix and list matching commits.
	if !strings.Contains(actionable.Cause, "abcd") {
		t.Errorf("error cause should mention prefix 'abcd', got: %q", actionable.Cause)
	}
	if !strings.Contains(actionable.Cause, "3 snapshots") {
		t.Errorf("error cause should mention '3 snapshots', got: %q", actionable.Cause)
	}
}

func TestResolve_unknownRef(t *testing.T) {
	dir := t.TempDir()

	idx := Index{}

	// Test various invalid refs.
	invalidRefs := []string{"foo", "bar-baz", "~", "~abc", "HEAD", "main"}
	for _, ref := range invalidRefs {
		_, _, err := Resolve(dir, ref, idx)
		if err == nil {
			t.Errorf("expected error for ref %q, got nil", ref)
			continue
		}

		actionable, ok := err.(failures.Actionable)
		if !ok {
			t.Errorf("ref %q: expected failures.Actionable, got %T: %v", ref, err, err)
			continue
		}
		if actionable.Title != "Snapshot not found" {
			t.Errorf("ref %q: error title = %q, want %q", ref, actionable.Title, "Snapshot not found")
		}
		if actionable.Next != "rg diff --list" {
			t.Errorf("ref %q: error next = %q, want %q", ref, actionable.Next, "rg diff --list")
		}
	}
}

func TestResolve_fileMissingFromDisk(t *testing.T) {
	dir := t.TempDir()

	// Create an archived snapshot.
	baseTime := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	snap := makeTestSnapshot(t, baseTime, "abc1234")
	if err := Archive(dir, snap); err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	// Delete the archive file from disk.
	archivePath := filepath.Join(HistoryDir(dir), idx.Entries[0].File)
	if err := os.Remove(archivePath); err != nil {
		t.Fatalf("failed to remove archive file: %v", err)
	}

	// Try to resolve ~1 — file is missing.
	_, _, err = Resolve(dir, "~1", idx)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}

	actionable, ok := err.(failures.Actionable)
	if !ok {
		t.Fatalf("expected failures.Actionable, got %T: %v", err, err)
	}
	if actionable.Title != "Snapshot file missing" {
		t.Errorf("error title = %q, want %q", actionable.Title, "Snapshot file missing")
	}
	if actionable.Next != "rg snapshot" {
		t.Errorf("error next = %q, want %q", actionable.Next, "rg snapshot")
	}
}


// --- List tests ---

func TestList_includesLatestAndHistory(t *testing.T) {
	dir := t.TempDir()

	// Create a current snapshot.json.
	now := time.Date(2026, 5, 20, 13, 31, 38, 0, time.UTC)
	currentSnap := makeTestSnapshot(t, now, "current1")
	if err := snapshot.Write(dir, currentSnap); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}

	// Create 3 archived snapshots.
	baseTime := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		snap := makeTestSnapshot(t, baseTime.Add(time.Duration(i)*24*time.Hour), fmt.Sprintf("hist%04d", i))
		if err := Archive(dir, snap); err != nil {
			t.Fatalf("Archive %d failed: %v", i, err)
		}
	}

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	// Should have 4 entries: latest + 3 history.
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	// First entry should be "latest" with the current snapshot's data.
	if entries[0].Ref != "latest" {
		t.Errorf("entries[0].Ref = %q, want %q", entries[0].Ref, "latest")
	}
	if entries[0].GitCommit != "current1" {
		t.Errorf("entries[0].GitCommit = %q, want %q", entries[0].GitCommit, "current1")
	}
	if !entries[0].CreatedAt.Equal(now) {
		t.Errorf("entries[0].CreatedAt = %v, want %v", entries[0].CreatedAt, now)
	}
	if entries[0].TestPassed != 4 {
		t.Errorf("entries[0].TestPassed = %d, want 4", entries[0].TestPassed)
	}

	// History entries should be ~1, ~2, ~3 (newest-first from index).
	if entries[1].Ref != "~1" {
		t.Errorf("entries[1].Ref = %q, want %q", entries[1].Ref, "~1")
	}
	if entries[2].Ref != "~2" {
		t.Errorf("entries[2].Ref = %q, want %q", entries[2].Ref, "~2")
	}
	if entries[3].Ref != "~3" {
		t.Errorf("entries[3].Ref = %q, want %q", entries[3].Ref, "~3")
	}

	// ~1 should be the newest archived (hist0002, since index is newest-first).
	if entries[1].GitCommit != "hist0002" {
		t.Errorf("entries[1].GitCommit = %q, want %q", entries[1].GitCommit, "hist0002")
	}
	// ~3 should be the oldest archived (hist0000).
	if entries[3].GitCommit != "hist0000" {
		t.Errorf("entries[3].GitCommit = %q, want %q", entries[3].GitCommit, "hist0000")
	}
}

func TestList_emptyHistoryOnlyLatest(t *testing.T) {
	dir := t.TempDir()

	// Create only a current snapshot.json (no history).
	now := time.Date(2026, 5, 20, 13, 31, 38, 0, time.UTC)
	snap := makeTestSnapshot(t, now, "onlysnap")
	if err := snapshot.Write(dir, snap); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	// Should have exactly 1 entry: latest.
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Ref != "latest" {
		t.Errorf("entries[0].Ref = %q, want %q", entries[0].Ref, "latest")
	}
	if entries[0].GitCommit != "onlysnap" {
		t.Errorf("entries[0].GitCommit = %q, want %q", entries[0].GitCommit, "onlysnap")
	}
	if entries[0].TestPassed != 4 {
		t.Errorf("entries[0].TestPassed = %d, want 4", entries[0].TestPassed)
	}
}

func TestList_noSnapshotReturnsError(t *testing.T) {
	dir := t.TempDir()

	// No snapshot.json exists at all.
	_, err := List(dir)
	if err == nil {
		t.Fatal("expected error when no snapshot exists, got nil")
	}

	actionable, ok := err.(failures.Actionable)
	if !ok {
		t.Fatalf("expected failures.Actionable, got %T: %v", err, err)
	}
	if actionable.Title != "No snapshot available" {
		t.Errorf("error title = %q, want %q", actionable.Title, "No snapshot available")
	}
	if actionable.Next != "rg snapshot" {
		t.Errorf("error next = %q, want %q", actionable.Next, "rg snapshot")
	}
}
