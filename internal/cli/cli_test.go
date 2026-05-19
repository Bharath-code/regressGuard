package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootHelpIsCompact(t *testing.T) {
	cmd := NewRootCommand(BuildInfo{Version: "test", Commit: "abc", Date: "today"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("help returned error: %v", err)
	}

	help := out.String()
	for _, want := range []string{
		"RegressGuard",
		"Before you commit, know what broke.",
		"Commands:",
		"Start:",
		"rg init",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("root help missing %q:\n%s", want, help)
		}
	}
	if strings.Contains(help, "Flags:") {
		t.Fatalf("root help should not dump flags:\n%s", help)
	}
	assertMaxLineWidth(t, help, 80)
}

func TestCommandHelpIncludesContractSections(t *testing.T) {
	commands := [][]string{
		{"init", "--help"},
		{"snapshot", "--help"},
		{"check", "--help"},
		{"doctor", "--help"},
		{"version", "--help"},
		{"hook", "--help"},
		{"config", "--help"},
	}

	for _, args := range commands {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := NewRootCommand(BuildInfo{Version: "test", Commit: "abc", Date: "today"})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs(args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("help returned error: %v", err)
			}

			help := out.String()
			for _, want := range []string{"Usage:", "Examples:", "Exit codes:"} {
				if !strings.Contains(help, want) {
					t.Fatalf("command help missing %q:\n%s", want, help)
				}
			}
			assertMaxLineWidth(t, help, 80)
		})
	}
}

func TestJSONModeWritesOnlyJSONToStdout(t *testing.T) {
	// Set up a temp dir with a valid config and snapshot so rg check --json
	// runs through the real engine and produces valid JSON on stdout.
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.MkdirAll(".regressguard", 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a minimal config.
	configJSON := `{
  "version": 1,
  "testCommand": "echo 'Tests  3 passed'",
  "serverUrl": "http://localhost:19999",
  "routes": []
}
`
	if err := os.WriteFile(".regressguard/config.json", []byte(configJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a minimal snapshot.
	snapJSON := `{
  "version": 1,
  "createdAt": "2026-05-19T00:00:00Z",
  "gitCommit": "abc1234",
  "tests": {"passed": 3, "failed": 0, "skipped": 0, "durationMs": 0},
  "routes": {}
}
`
	if err := os.WriteFile(".regressguard/snapshot.json", []byte(snapJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand(BuildInfo{Version: "test", Commit: "abc", Date: "today"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"check", "--json", "--verbose"})

	// Run may exit 0 (pass/warning) or 1 (critical) — we only care about JSON validity.
	_ = cmd.Execute()

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	// Status must be one of the real statuses.
	status, _ := payload["status"].(string)
	validStatuses := map[string]bool{"pass": true, "warning": true, "critical": true}
	if !validStatuses[status] {
		t.Fatalf("unexpected JSON status: %q", status)
	}
	// Verbose diagnostics must go to stderr, not stdout.
	if strings.Contains(stderr.String(), "{") {
		t.Fatalf("stderr should not contain JSON, got: %q", stderr.String())
	}
}

func TestMissingSnapshotIsActionable(t *testing.T) {
	// Set up a temp dir with a valid config but no snapshot.
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.MkdirAll(".regressguard", 0o755); err != nil {
		t.Fatal(err)
	}
	configJSON := `{
  "version": 1,
  "testCommand": "echo ok",
  "serverUrl": "http://localhost:3000",
  "routes": []
}
`
	if err := os.WriteFile(".regressguard/config.json", []byte(configJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand(BuildInfo{Version: "test", Commit: "abc", Date: "today"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"check"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing snapshot error")
	}
	message := err.Error()
	for _, want := range []string{
		"rg check failed: no snapshot found.",
		"Likely cause:",
		"Run:",
		"rg snapshot",
		"Need more context:",
		"rg check --help",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("missing %q in actionable error:\n%s", want, message)
		}
	}
	if stdout.Len() != 0 {
		t.Fatalf("human error should not write stdout, got: %q", stdout.String())
	}
}

func TestMissingSnapshotJSONIsParseable(t *testing.T) {
	// Set up a temp dir with a valid config but no snapshot.
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.MkdirAll(".regressguard", 0o755); err != nil {
		t.Fatal(err)
	}
	configJSON := `{
  "version": 1,
  "testCommand": "echo ok",
  "serverUrl": "http://localhost:3000",
  "routes": []
}
`
	if err := os.WriteFile(".regressguard/config.json", []byte(configJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand(BuildInfo{Version: "test", Commit: "abc", Date: "today"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"check", "--json"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected missing snapshot exit error")
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if payload["status"] != "error" {
		t.Fatalf("unexpected status: %#v", payload["status"])
	}
}

func assertMaxLineWidth(t *testing.T, text string, max int) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if len(line) > max {
			t.Fatalf("line exceeds %d columns (%d): %q", max, len(line), line)
		}
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(abs); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})
}
