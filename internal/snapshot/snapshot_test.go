package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWrite_byteStableWhenContractUnchanged(t *testing.T) {
	d := t.TempDir()
	mk := func(ms int64, at time.Time) Snapshot {
		return Snapshot{Version: Version, CreatedAt: at, GitCommit: "x", Tests: TestSummary{Passed: 3, DurationMs: ms},
			Routes: map[string]RouteRecord{"GET /a": {Method: "GET", Path: "/a", Status: 200, SchemaHash: "h", MS: ms}}}
	}
	if err := Write(d, mk(10, time.Now())); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(Path(d))
	if err := Write(d, mk(99, time.Now().Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(Path(d))
	if string(first) != string(second) {
		t.Error("snapshot rewritten despite unchanged contract")
	}
	changed := mk(10, time.Now())
	changed.Routes["GET /a"] = RouteRecord{Method: "GET", Path: "/a", Status: 500, SchemaHash: "h"}
	_ = Write(d, changed)
	third, _ := os.ReadFile(Path(d))
	if string(third) == string(first) {
		t.Error("snapshot not rewritten after contract change")
	}
}

func TestEnsureGitignore(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, ".gitignore")
	os.WriteFile(p, []byte("bin/\n.regressguard/\n"), 0o644)
	if !LegacyIgnore(d) {
		t.Fatal("legacy ignore not detected")
	}
	for i := 0; i < 2; i++ {
		if err := EnsureGitignore(d); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(p)
	if string(b) != "bin/\n.regressguard/*\n!.regressguard/snapshot.json\n" {
		t.Errorf("got %q", b)
	}
	if LegacyIgnore(d) {
		t.Error("still legacy after fix")
	}
}
