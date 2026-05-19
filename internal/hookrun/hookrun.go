// Package hookrun implements rg hook install and rg hook uninstall.
// It manages a clearly-delimited block inside .git/hooks/pre-commit so it
// composes safely with husky, lint-staged, and any other hook managers.
package hookrun

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// blockBegin and blockEnd delimit the RegressGuard-managed section of the
	// pre-commit hook. Everything between these markers is owned by rg.
	blockBegin = "# --- RegressGuard begin ---"
	blockEnd   = "# --- RegressGuard end ---"

	// hookScript is the shell code injected between the markers.
	// It runs rg check in hook mode and blocks the commit on exit code 1.
	hookScript = `RG_HOOK=1 rg check
RG_EXIT=$?
if [ $RG_EXIT -eq 1 ]; then
  echo ""
  echo "Commit blocked. Use --no-verify only if you accept the risk."
  exit 1
fi`

	hookPreamble = `#!/bin/sh
`
)

// InstallOptions configures rg hook install.
type InstallOptions struct {
	// GitDir is the path to the .git directory. Defaults to ".git".
	GitDir string
	// ProjectRoot is used to detect hook managers. Defaults to ".".
	ProjectRoot string
	Stdout      io.Writer
	Stderr      io.Writer
}

// UninstallOptions configures rg hook uninstall.
type UninstallOptions struct {
	GitDir string
	Stdout io.Writer
	Stderr io.Writer
}

// Install creates or updates .git/hooks/pre-commit with the RegressGuard block.
// If the hook file already exists it is preserved; the block is appended or
// replaced in-place. Returns the path to the hook file.
func Install(opts InstallOptions) (string, error) {
	opts = installDefaults(opts)

	hookPath := filepath.Join(opts.GitDir, "hooks", "pre-commit")

	// Detect hook managers and print guidance before writing.
	if manager := detectHookManager(opts.ProjectRoot); manager != "" {
		_, _ = fmt.Fprintf(opts.Stdout, "%s\n", hookManagerGuidance(manager, hookPath))
	}

	// Read existing hook content (may not exist yet).
	existing, err := readFileOrEmpty(hookPath)
	if err != nil {
		return "", fmt.Errorf("read pre-commit hook: %w", err)
	}

	// Build the new content.
	updated := injectBlock(existing, managedBlock())

	// Ensure the hooks directory exists.
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return "", fmt.Errorf("create hooks dir: %w", err)
	}

	// Write the hook file.
	if err := os.WriteFile(hookPath, []byte(updated), 0o755); err != nil {
		return "", fmt.Errorf("write pre-commit hook: %w", err)
	}

	_, _ = fmt.Fprintf(opts.Stdout, "OK Installed pre-commit hook\n")
	_, _ = fmt.Fprintf(opts.Stdout, "   %s\n", hookPath)
	_, _ = fmt.Fprintf(opts.Stdout, "\n")
	_, _ = fmt.Fprintf(opts.Stdout, "Behavior:\n")
	_, _ = fmt.Fprintf(opts.Stdout, "  rg check runs before every commit.\n")
	_, _ = fmt.Fprintf(opts.Stdout, "  Critical regressions block the commit.\n")
	_, _ = fmt.Fprintf(opts.Stdout, "  Warnings allow the commit through.\n")
	_, _ = fmt.Fprintf(opts.Stdout, "\n")
	_, _ = fmt.Fprintf(opts.Stdout, "Bypass (emergencies only):\n")
	_, _ = fmt.Fprintf(opts.Stdout, "  git commit --no-verify\n")
	_, _ = fmt.Fprintf(opts.Stdout, "\n")
	_, _ = fmt.Fprintf(opts.Stdout, "Uninstall:\n")
	_, _ = fmt.Fprintf(opts.Stdout, "  rg hook uninstall\n")

	return hookPath, nil
}

// Uninstall removes the RegressGuard-managed block from .git/hooks/pre-commit.
// If the file becomes empty (or only a shebang) after removal, it is deleted.
func Uninstall(opts UninstallOptions) error {
	opts = uninstallDefaults(opts)

	hookPath := filepath.Join(opts.GitDir, "hooks", "pre-commit")

	existing, err := readFileOrEmpty(hookPath)
	if err != nil {
		return fmt.Errorf("read pre-commit hook: %w", err)
	}

	if !containsBlock(existing) {
		_, _ = fmt.Fprintf(opts.Stdout, "i No RegressGuard hook block found in %s\n", hookPath)
		return nil
	}

	updated := removeBlock(existing)

	// If nothing meaningful remains, delete the file.
	if isEmptyHook(updated) {
		if err := os.Remove(hookPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove hook file: %w", err)
		}
		_, _ = fmt.Fprintf(opts.Stdout, "OK Removed pre-commit hook\n")
		_, _ = fmt.Fprintf(opts.Stdout, "   %s deleted (was empty after removal)\n", hookPath)
		return nil
	}

	if err := os.WriteFile(hookPath, []byte(updated), 0o755); err != nil {
		return fmt.Errorf("write pre-commit hook: %w", err)
	}

	_, _ = fmt.Fprintf(opts.Stdout, "OK Removed RegressGuard block from %s\n", hookPath)
	return nil
}

// managedBlock returns the full text of the RegressGuard-managed block.
func managedBlock() string {
	return blockBegin + "\n" + hookScript + "\n" + blockEnd + "\n"
}

// injectBlock inserts or replaces the managed block in existing hook content.
// If the file is empty it adds a shebang first.
func injectBlock(existing, block string) string {
	// Replace existing block if present.
	if containsBlock(existing) {
		return replaceBlock(existing, block)
	}

	// New file — start with shebang if not already present.
	if existing == "" {
		return hookPreamble + block
	}

	// Append to existing content (preserve other hooks).
	if !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	return existing + "\n" + block
}

// containsBlock reports whether the content contains the managed block markers.
func containsBlock(content string) bool {
	return strings.Contains(content, blockBegin) && strings.Contains(content, blockEnd)
}

// replaceBlock replaces the content between (and including) the markers.
func replaceBlock(content, newBlock string) string {
	start := strings.Index(content, blockBegin)
	end := strings.Index(content, blockEnd)
	if start == -1 || end == -1 {
		return content
	}
	end += len(blockEnd)
	// Include trailing newline if present.
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return content[:start] + newBlock + content[end:]
}

// removeBlock strips the managed block from content.
func removeBlock(content string) string {
	start := strings.Index(content, blockBegin)
	end := strings.Index(content, blockEnd)
	if start == -1 || end == -1 {
		return content
	}
	end += len(blockEnd)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	result := content[:start] + content[end:]
	// Clean up double blank lines left behind.
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	return result
}

// isEmptyHook reports whether the hook content is effectively empty
// (only whitespace or a bare shebang line).
func isEmptyHook(content string) bool {
	trimmed := strings.TrimSpace(content)
	return trimmed == "" || trimmed == "#!/bin/sh" || trimmed == "#!/bin/bash"
}

func readFileOrEmpty(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// detectHookManager checks for common hook managers in the project root.
func detectHookManager(root string) string {
	// husky: .husky directory or husky config in package.json
	if _, err := os.Stat(filepath.Join(root, ".husky")); err == nil {
		return "husky"
	}
	// lint-staged: .lintstagedrc or lint-staged key (detected via presence)
	for _, name := range []string{".lintstagedrc", ".lintstagedrc.json", ".lintstagedrc.js"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return "lint-staged"
		}
	}
	return ""
}

// hookManagerGuidance returns a human-readable note about composing with a
// detected hook manager.
func hookManagerGuidance(manager, hookPath string) string {
	switch manager {
	case "husky":
		return fmt.Sprintf(
			"i Detected husky. RegressGuard will be added to %s.\n"+
				"  If you use husky's own pre-commit file, also add:\n"+
				"    rg check\n"+
				"  to .husky/pre-commit so it runs in both contexts.",
			hookPath,
		)
	case "lint-staged":
		return fmt.Sprintf(
			"i Detected lint-staged. RegressGuard block added to %s.\n"+
				"  lint-staged runs inside the hook — rg check will run alongside it.",
			hookPath,
		)
	default:
		return ""
	}
}

func installDefaults(opts InstallOptions) InstallOptions {
	if opts.GitDir == "" {
		opts.GitDir = ".git"
	}
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = "."
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return opts
}

func uninstallDefaults(opts UninstallOptions) UninstallOptions {
	if opts.GitDir == "" {
		opts.GitDir = ".git"
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return opts
}
