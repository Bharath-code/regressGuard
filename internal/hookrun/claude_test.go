package hookrun

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readSettings(t *testing.T, root string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("settings not valid JSON: %v", err)
	}
	return m
}

func stopGroups(t *testing.T, m map[string]any) []any {
	t.Helper()
	hooks, _ := m["hooks"].(map[string]any)
	groups, _ := hooks["Stop"].([]any)
	return groups
}

func TestInstallClaude_createsSettings(t *testing.T) {
	root := t.TempDir()
	if _, err := InstallClaude(ClaudeOptions{ProjectRoot: root, Stdout: &bytes.Buffer{}}); err != nil {
		t.Fatalf("InstallClaude: %v", err)
	}
	if n := len(stopGroups(t, readSettings(t, root))); n != 1 {
		t.Fatalf("want 1 Stop group, got %d", n)
	}
}

func TestInstallClaude_idempotent(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 2; i++ {
		if _, err := InstallClaude(ClaudeOptions{ProjectRoot: root, Stdout: &bytes.Buffer{}}); err != nil {
			t.Fatalf("InstallClaude #%d: %v", i, err)
		}
	}
	if n := len(stopGroups(t, readSettings(t, root))); n != 1 {
		t.Fatalf("want 1 Stop group after two installs, got %d", n)
	}
}

func TestInstallClaude_preservesExisting(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude")
	_ = os.MkdirAll(dir, 0o755)
	existing := `{"permissions":{"allow":["Bash(go test:*)"]},"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo mine"}]}],"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo pre"}]}]}}`
	_ = os.WriteFile(filepath.Join(dir, "settings.json"), []byte(existing), 0o644)

	if _, err := InstallClaude(ClaudeOptions{ProjectRoot: root, Stdout: &bytes.Buffer{}}); err != nil {
		t.Fatalf("InstallClaude: %v", err)
	}
	m := readSettings(t, root)
	if _, ok := m["permissions"]; !ok {
		t.Error("permissions key dropped")
	}
	hooks := m["hooks"].(map[string]any)
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("PreToolUse hooks dropped")
	}
	if n := len(stopGroups(t, m)); n != 2 {
		t.Errorf("want user's Stop group + ours = 2, got %d", n)
	}
}

func TestInstallClaude_refusesInvalidJSON(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude")
	_ = os.MkdirAll(dir, 0o755)
	p := filepath.Join(dir, "settings.json")
	_ = os.WriteFile(p, []byte("{not json"), 0o644)
	if _, err := InstallClaude(ClaudeOptions{ProjectRoot: root, Stdout: &bytes.Buffer{}}); err == nil {
		t.Fatal("want error on invalid settings.json")
	}
	if b, _ := os.ReadFile(p); string(b) != "{not json" {
		t.Error("invalid settings.json must be left untouched")
	}
}
