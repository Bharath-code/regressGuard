// Package state manages local persistent state for RegressGuard.
// State is stored in .regressguard/state.json and tracks things like
// whether one-time nudges have been shown, check streak counts, etc.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const FileName = "state.json"

// State holds persistent local state that survives across runs.
type State struct {
	HookNudgeShown bool `json:"hookNudgeShown,omitempty"`
	CheckStreak    int  `json:"checkStreak,omitempty"`
	FirstPassShown bool `json:"firstPassShown,omitempty"`
}

// Path returns the absolute path to state.json given a project root.
func Path(root string) string {
	return filepath.Join(root, ".regressguard", FileName)
}

// Load reads the state file. Returns a zero State if the file doesn't exist.
func Load(root string) State {
	data, err := os.ReadFile(Path(root))
	if err != nil {
		return State{}
	}
	var s State
	_ = json.Unmarshal(data, &s)
	return s
}

// Save writes the state file to disk.
func Save(root string, s State) error {
	dir := filepath.Join(root, ".regressguard")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(Path(root), data, 0o644)
}
