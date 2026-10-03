package state

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.ts"), []byte("v1"), 0o644)
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "base"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestTreeFingerprint(t *testing.T) {
	dir := gitRepo(t)
	base := TreeFingerprint(dir)
	if base == "" {
		t.Fatal("git repo should fingerprint")
	}
	if TreeFingerprint(dir) != base {
		t.Fatal("fingerprint must be stable")
	}
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".regressguard/*\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, ".regressguard"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".regressguard", "snapshot.json"), []byte("{}"), 0o644)
	if TreeFingerprint(dir) != base {
		t.Fatal(".gitignore and .regressguard/ must not affect the fingerprint")
	}
	_ = os.WriteFile(filepath.Join(dir, "a.ts"), []byte("v2"), 0o644)
	edited := TreeFingerprint(dir)
	if edited == base {
		t.Fatal("edit must change the fingerprint")
	}
	_ = os.WriteFile(filepath.Join(dir, "b.ts"), []byte("x"), 0o644)
	if TreeFingerprint(dir) == edited {
		t.Fatal("new untracked file must change the fingerprint")
	}
}

func TestTreeFingerprint_noGitIsUnknown(t *testing.T) {
	if TreeFingerprint(t.TempDir()) != "" {
		t.Fatal("outside git the fingerprint must be empty (fail closed)")
	}
}
