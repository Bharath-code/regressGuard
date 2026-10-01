package hookrun

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// claudeMarker tags the hook command so re-installs find and replace our entry.
const claudeMarker = "RG_CLAUDE_STOP=1"

// ClaudeOptions configures regressguard hook install --claude.
type ClaudeOptions struct {
	// ProjectRoot holds .claude/settings.json. Defaults to ".".
	ProjectRoot string
	Stdout      io.Writer
}

// claudeCommand blocks the agent's Stop (exit 2, diff on stderr) only on a
// CRITICAL result (check exit 1). Operational errors (exit 2) never block:
// a broken dev server must not trap the agent in a stop loop.
func claudeCommand(bin string) string {
	return fmt.Sprintf(`%s "%s" check --json >&2; [ $? -eq 1 ] && exit 2 || exit 0`, claudeMarker, bin)
}

// InstallClaude merges a Stop hook into project-scope .claude/settings.json.
// Existing settings and hooks are preserved; re-running replaces our entry.
func InstallClaude(opts ClaudeOptions) (string, error) {
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = "."
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	path := filepath.Join(opts.ProjectRoot, ".claude", "settings.json")

	settings := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return "", fmt.Errorf("%s is not valid JSON (left untouched): %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	groups, _ := hooks["Stop"].([]any)
	kept := make([]any, 0, len(groups)+1)
	for _, g := range groups {
		if !isOurGroup(g) {
			kept = append(kept, g)
		}
	}
	kept = append(kept, map[string]any{
		"hooks": []any{map[string]any{"type": "command", "command": claudeCommand(resolveBinaryPath())}},
	})
	hooks["Stop"] = kept
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return "", err
	}
	_, _ = fmt.Fprintf(opts.Stdout, "Installed Claude Code Stop hook: %s\nThe agent can't finish while `regressguard check` is CRITICAL.\n", path)
	return path, nil
}

func isOurGroup(g any) bool {
	gm, _ := g.(map[string]any)
	hs, _ := gm["hooks"].([]any)
	for _, h := range hs {
		hm, _ := h.(map[string]any)
		if cmd, _ := hm["command"].(string); strings.Contains(cmd, claudeMarker) {
			return true
		}
	}
	return false
}
