package doctorrun

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctor_flagsStaleBareRGHook(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := "#!/bin/sh\n# --- RegressGuard begin ---\nRG_HOOK=1 rg check\n# --- RegressGuard end ---\n"
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(stale), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	ok := Run(Options{ProjectRoot: dir, Stdout: &stdout, Stderr: &stdout})

	if ok {
		t.Error("doctor should fail when the hook uses bare 'rg' (ripgrep collision)")
	}
	out := stdout.String()
	if !strings.Contains(out, "bare 'rg'") {
		t.Errorf("expected stale-hook diagnosis in output\nGot:\n%s", out)
	}
	if !strings.Contains(out, "rg hook install") {
		t.Errorf("expected reinstall fix in output\nGot:\n%s", out)
	}
}

func TestDoctor_okWithFixedHook(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fixed := "#!/bin/sh\n# --- RegressGuard begin ---\nRG_BIN=\"/usr/local/bin/regressguard\"\nRG_HOOK=1 \"$RG_BIN\" check\n# --- RegressGuard end ---\n"
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(fixed), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	Run(Options{ProjectRoot: dir, Stdout: &stdout, Stderr: &stdout})

	if strings.Contains(stdout.String(), "bare 'rg'") {
		t.Errorf("absolute-path hook should not be flagged\nGot:\n%s", stdout.String())
	}
}
