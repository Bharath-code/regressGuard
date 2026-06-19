package hookrun

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- helpers ---

func makeGitDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755); err != nil {
		t.Fatalf("create .git/hooks: %v", err)
	}
	return dir
}

func readHook(t *testing.T, gitDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(gitDir, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	return string(data)
}

func hookExists(gitDir string) bool {
	_, err := os.Stat(filepath.Join(gitDir, ".git", "hooks", "pre-commit"))
	return err == nil
}

// --- E5-T1: install hook ---

func TestInstall_createsHookFile(t *testing.T) {
	dir := makeGitDir(t)
	var stdout bytes.Buffer

	hookPath, err := Install(InstallOptions{
		GitDir:      filepath.Join(dir, ".git"),
		ProjectRoot: dir,
		Stdout:      &stdout,
	})
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}

	if !hookExists(dir) {
		t.Error("expected pre-commit hook file to be created")
	}

	content := readHook(t, dir)
	if !strings.Contains(content, blockBegin) {
		t.Error("hook missing begin marker")
	}
	if !strings.Contains(content, blockEnd) {
		t.Error("hook missing end marker")
	}
	if !strings.Contains(content, "check") {
		t.Error("hook missing 'check' command")
	}
	if !strings.Contains(content, "#!/bin/sh") {
		t.Error("hook missing shebang")
	}

	// Hook file must be executable.
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("stat hook: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("hook file should be executable")
	}
}

func TestInstall_outputMentionsHookPath(t *testing.T) {
	dir := makeGitDir(t)
	var stdout bytes.Buffer

	_, err := Install(InstallOptions{
		GitDir:      filepath.Join(dir, ".git"),
		ProjectRoot: dir,
		Stdout:      &stdout,
	})
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"OK", "pre-commit", "check", "Bypass", "--no-verify", "rg hook uninstall"} {
		if !strings.Contains(out, want) {
			t.Errorf("install output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestInstall_idempotent(t *testing.T) {
	dir := makeGitDir(t)
	var stdout bytes.Buffer

	opts := InstallOptions{
		GitDir:      filepath.Join(dir, ".git"),
		ProjectRoot: dir,
		Stdout:      &stdout,
	}

	// Install twice.
	if _, err := Install(opts); err != nil {
		t.Fatalf("first Install error: %v", err)
	}
	if _, err := Install(opts); err != nil {
		t.Fatalf("second Install error: %v", err)
	}

	content := readHook(t, dir)

	// Block should appear exactly once.
	count := strings.Count(content, blockBegin)
	if count != 1 {
		t.Errorf("expected block to appear once, got %d times", count)
	}
}

func TestInstall_composesWithExistingHook(t *testing.T) {
	dir := makeGitDir(t)

	// Write an existing hook with other content.
	existing := "#!/bin/sh\nnpm run lint\n"
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte(existing), 0o755); err != nil {
		t.Fatalf("write existing hook: %v", err)
	}

	var stdout bytes.Buffer
	if _, err := Install(InstallOptions{
		GitDir:      filepath.Join(dir, ".git"),
		ProjectRoot: dir,
		Stdout:      &stdout,
	}); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	content := readHook(t, dir)

	// Original content must be preserved.
	if !strings.Contains(content, "npm run lint") {
		t.Error("existing hook content should be preserved")
	}
	// RegressGuard block must be present.
	if !strings.Contains(content, blockBegin) {
		t.Error("hook missing RegressGuard block")
	}
}

// --- E5-T2: husky/lint-staged compatibility ---

func TestInstall_detectsHusky(t *testing.T) {
	dir := makeGitDir(t)

	// Create .husky directory to simulate husky setup.
	if err := os.MkdirAll(filepath.Join(dir, ".husky"), 0o755); err != nil {
		t.Fatalf("create .husky: %v", err)
	}

	var stdout bytes.Buffer
	if _, err := Install(InstallOptions{
		GitDir:      filepath.Join(dir, ".git"),
		ProjectRoot: dir,
		Stdout:      &stdout,
	}); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "husky") {
		t.Errorf("expected husky guidance in output\nGot:\n%s", out)
	}
}

func TestInstall_detectsLintStaged(t *testing.T) {
	dir := makeGitDir(t)

	// Create .lintstagedrc to simulate lint-staged setup.
	if err := os.WriteFile(filepath.Join(dir, ".lintstagedrc"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("create .lintstagedrc: %v", err)
	}

	var stdout bytes.Buffer
	if _, err := Install(InstallOptions{
		GitDir:      filepath.Join(dir, ".git"),
		ProjectRoot: dir,
		Stdout:      &stdout,
	}); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "lint-staged") {
		t.Errorf("expected lint-staged guidance in output\nGot:\n%s", out)
	}
}

// --- E5-T3: hook check execution (script content) ---

func TestInstall_hookScriptBlocksOnExit1(t *testing.T) {
	dir := makeGitDir(t)
	var stdout bytes.Buffer

	if _, err := Install(InstallOptions{
		GitDir:      filepath.Join(dir, ".git"),
		ProjectRoot: dir,
		Stdout:      &stdout,
	}); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	content := readHook(t, dir)

	// The script must capture rg check exit code and block on 1.
	if !strings.Contains(content, "RG_EXIT=$?") {
		t.Error("hook should capture check exit code")
	}
	if !strings.Contains(content, "exit 1") {
		t.Error("hook should exit 1 on critical regression")
	}
	if !strings.Contains(content, "--no-verify") {
		t.Error("hook should mention --no-verify bypass")
	}
}

func TestInstall_hookScriptUsesAbsolutePath(t *testing.T) {
	dir := makeGitDir(t)
	var stdout bytes.Buffer

	if _, err := Install(InstallOptions{
		GitDir:      filepath.Join(dir, ".git"),
		ProjectRoot: dir,
		Stdout:      &stdout,
	}); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	content := readHook(t, dir)
	if strings.Contains(content, "RG_HOOK=1 rg check") {
		t.Error("hook should not use bare 'rg check' — must use absolute path")
	}
	hasAbsPath := strings.Contains(content, "RG_HOOK=1 /") || strings.Contains(content, "RG_HOOK=1 regressguard")
	if !hasAbsPath {
		t.Errorf("hook should use absolute binary path or 'regressguard' fallback\nGot:\n%s", content)
	}
}

func TestInstall_warnsOnRipgrepConflict(t *testing.T) {
	dir := makeGitDir(t)
	var stdout bytes.Buffer

	if _, err := Install(InstallOptions{
		GitDir:      filepath.Join(dir, ".git"),
		ProjectRoot: dir,
		Stdout:      &stdout,
	}); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	if isRipgrepOnPath() {
		out := stdout.String()
		if !strings.Contains(out, "ripgrep") {
			t.Errorf("expected ripgrep conflict warning\nGot:\n%s", out)
		}
	}
}

// --- E5-T4: hook output (Flow I) ---

func TestInstall_outputFitsViewport(t *testing.T) {
	dir := makeGitDir(t)
	var stdout bytes.Buffer

	if _, err := Install(InstallOptions{
		GitDir:      filepath.Join(dir, ".git"),
		ProjectRoot: dir,
		Stdout:      &stdout,
	}); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	for _, line := range strings.Split(stdout.String(), "\n") {
		// Skip the indented path line — it contains a temp dir path that can be long.
		// In production the path is short (.git/hooks/pre-commit).
		if strings.HasPrefix(strings.TrimSpace(line), "/") || strings.HasPrefix(strings.TrimSpace(line), ".git") {
			continue
		}
		if len(line) > 80 {
			t.Errorf("output line exceeds 80 columns (%d): %q", len(line), line)
		}
	}
}

// --- E5-T5: uninstall hook ---

func TestUninstall_removesBlock(t *testing.T) {
	dir := makeGitDir(t)
	var stdout bytes.Buffer

	// Install first.
	if _, err := Install(InstallOptions{
		GitDir:      filepath.Join(dir, ".git"),
		ProjectRoot: dir,
		Stdout:      &stdout,
	}); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	stdout.Reset()

	// Uninstall.
	if err := Uninstall(UninstallOptions{
		GitDir: filepath.Join(dir, ".git"),
		Stdout: &stdout,
	}); err != nil {
		t.Fatalf("Uninstall error: %v", err)
	}

	// Hook file should be gone (was only the RegressGuard block).
	if hookExists(dir) {
		t.Error("expected hook file to be deleted after uninstall")
	}

	out := stdout.String()
	if !strings.Contains(out, "OK") {
		t.Errorf("uninstall output missing 'OK'\nGot:\n%s", out)
	}
}

func TestUninstall_preservesOtherHookContent(t *testing.T) {
	dir := makeGitDir(t)

	// Write a hook with existing content + RegressGuard block.
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	content := "#!/bin/sh\nnpm run lint\n\n" + managedBlock()
	if err := os.WriteFile(hookPath, []byte(content), 0o755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	var stdout bytes.Buffer
	if err := Uninstall(UninstallOptions{
		GitDir: filepath.Join(dir, ".git"),
		Stdout: &stdout,
	}); err != nil {
		t.Fatalf("Uninstall error: %v", err)
	}

	// File should still exist with the other content.
	if !hookExists(dir) {
		t.Error("hook file should still exist after partial uninstall")
	}

	remaining := readHook(t, dir)
	if strings.Contains(remaining, blockBegin) {
		t.Error("RegressGuard block should be removed")
	}
	if !strings.Contains(remaining, "npm run lint") {
		t.Error("other hook content should be preserved")
	}
}

func TestUninstall_noopWhenBlockAbsent(t *testing.T) {
	dir := makeGitDir(t)

	// Write a hook without the RegressGuard block.
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\nnpm run lint\n"), 0o755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	var stdout bytes.Buffer
	if err := Uninstall(UninstallOptions{
		GitDir: filepath.Join(dir, ".git"),
		Stdout: &stdout,
	}); err != nil {
		t.Fatalf("Uninstall error: %v", err)
	}

	// File should be unchanged.
	content := readHook(t, dir)
	if !strings.Contains(content, "npm run lint") {
		t.Error("hook content should be unchanged when block is absent")
	}

	out := stdout.String()
	if !strings.Contains(out, "No RegressGuard hook block found") {
		t.Errorf("expected 'No RegressGuard hook block found' message\nGot:\n%s", out)
	}
}

func TestUninstall_noopWhenFileAbsent(t *testing.T) {
	dir := makeGitDir(t)
	var stdout bytes.Buffer

	// No hook file exists — should not error.
	if err := Uninstall(UninstallOptions{
		GitDir: filepath.Join(dir, ".git"),
		Stdout: &stdout,
	}); err != nil {
		t.Fatalf("Uninstall should not error when hook file absent: %v", err)
	}
}

// --- internal helpers ---

func TestInjectBlock_emptyFile(t *testing.T) {
	result := injectBlock("", managedBlock())
	if !strings.HasPrefix(result, "#!/bin/sh") {
		t.Error("empty file should get shebang prepended")
	}
	if !strings.Contains(result, blockBegin) {
		t.Error("result should contain block")
	}
}

func TestInjectBlock_replacesExisting(t *testing.T) {
	original := "#!/bin/sh\n" + blockBegin + "\nold content\n" + blockEnd + "\n"
	result := injectBlock(original, managedBlock())
	if strings.Contains(result, "old content") {
		t.Error("old block content should be replaced")
	}
	if strings.Count(result, blockBegin) != 1 {
		t.Error("block should appear exactly once after replacement")
	}
}

func TestRemoveBlock_leavesOtherContent(t *testing.T) {
	content := "#!/bin/sh\nnpm run lint\n\n" + managedBlock() + "\necho done\n"
	result := removeBlock(content)
	if strings.Contains(result, blockBegin) {
		t.Error("block should be removed")
	}
	if !strings.Contains(result, "npm run lint") {
		t.Error("other content should remain")
	}
}

func TestIsEmptyHook(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"#!/bin/sh", true},
		{"#!/bin/bash", true},
		{"#!/bin/sh\n", true},
		{"#!/bin/sh\nnpm run lint\n", false},
		{"some content", false},
	}
	for _, tc := range cases {
		got := isEmptyHook(tc.input)
		if got != tc.want {
			t.Errorf("isEmptyHook(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
