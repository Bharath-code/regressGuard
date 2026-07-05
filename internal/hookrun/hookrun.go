// Package hookrun implements rg hook install and rg hook uninstall.
// It manages a clearly-delimited block inside .git/hooks/pre-commit so it
// composes safely with husky, lint-staged, and any other hook managers.
package hookrun

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Bharath-code/regressguard/internal/ui"
)

const (
	// blockBegin and blockEnd delimit the RegressGuard-managed section of the
	// pre-commit hook. Everything between these markers is owned by rg.
	blockBegin = "# --- RegressGuard begin ---"
	blockEnd   = "# --- RegressGuard end ---"

	// hookScript is the shell code injected between the markers.
	// It runs rg check in hook mode and blocks the commit on exit code 1.
	// The binary path is resolved at install time to avoid conflicts with
	// ripgrep (which also installs as `rg`). The sanity check must fail
	// LOUDLY: a stale path would otherwise exit 127, which the exit-code
	// branch below reads as "no regression" — the guard would fail open.
	// `</dev/null` prevents ripgrep from hanging on stdin if it is ever hit.
	hookScript = `RG_BIN="{{BIN}}"
if ! "$RG_BIN" version </dev/null 2>/dev/null | grep -q RegressGuard; then
  echo "RegressGuard: binary missing or not RegressGuard at: $RG_BIN" >&2
  echo "  Regression checks CANNOT run (is 'rg' resolving to ripgrep?)." >&2
  echo "  Fix: reinstall the hook with 'hook install' from the RegressGuard binary." >&2
  echo "  Bypass once (at your own risk): git commit --no-verify" >&2
  exit 1
fi
RG_HOOK=1 "$RG_BIN" check
RG_EXIT=$?
if [ $RG_EXIT -eq 1 ]; then
  echo ""
  echo "Commit blocked. Use --no-verify only if you accept the risk."
  exit 1
fi`

	hookPreamble = `#!/bin/sh
`
)

// resolveBinaryPath returns the absolute path to the RegressGuard binary.
// It tries (in order): the running binary via os.Executable, `regressguard`
// on PATH, and finally `rg` on PATH. If `rg` resolves to ripgrep, it falls
// back to `regressguard` or the os.Executable path.
func resolveBinaryPath() string {
	if exe, err := os.Executable(); err == nil {
		if abs, err := filepath.Abs(exe); err == nil {
			return abs
		}
		return exe
	}
	if path, err := exec.LookPath("regressguard"); err == nil {
		return path
	}
	if path, err := exec.LookPath("rg"); err == nil {
		if !IsRipgrep(path) {
			return path
		}
	}
	return "regressguard"
}

// IsRipgrep checks whether the binary at path is ripgrep.
func IsRipgrep(path string) bool {
	cmd := exec.Command(path, "--version")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "ripgrep")
}

// isRipgrepOnPath checks whether `rg` on PATH is ripgrep.
func isRipgrepOnPath() bool {
	path, err := exec.LookPath("rg")
	if err != nil {
		return false
	}
	return IsRipgrep(path)
}

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

	// Detect ripgrep conflict — warn the user.
	if isRipgrepOnPath() {
		_, _ = fmt.Fprintf(opts.Stdout, "%s ripgrep detected as `rg` on PATH.\n", ui.Paint(opts.Stdout, ui.ColorWarn, ui.SymbolWarning))
		_, _ = fmt.Fprintf(opts.Stdout, "  The hook uses the absolute RegressGuard path — it is safe.\n")
		_, _ = fmt.Fprintf(opts.Stdout, "  To run interactively, use the full binary path.\n")
		_, _ = fmt.Fprintf(opts.Stdout, "\n")
	}

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

	_, _ = fmt.Fprintf(opts.Stdout, "%s Installed pre-commit hook\n", ui.Paint(opts.Stdout, ui.ColorOK, "OK"))
	_, _ = fmt.Fprintf(opts.Stdout, "   %s\n", ui.Paint(opts.Stdout, ui.ColorMuted, hookPath))
	_, _ = fmt.Fprintf(opts.Stdout, "\n")
	_, _ = fmt.Fprintf(opts.Stdout, "Behavior:\n")
	_, _ = fmt.Fprintf(opts.Stdout, "  rg check runs before every commit.\n")
	_, _ = fmt.Fprintf(opts.Stdout, "  Critical regressions block the commit.\n")
	_, _ = fmt.Fprintf(opts.Stdout, "  Warnings allow the commit through.\n")
	_, _ = fmt.Fprintf(opts.Stdout, "\n")
	_, _ = fmt.Fprintf(opts.Stdout, "Bypass (emergencies only):\n")
	_, _ = fmt.Fprintf(opts.Stdout, "  %s\n", ui.Paint(opts.Stdout, ui.ColorInfo, "git commit --no-verify"))
	_, _ = fmt.Fprintf(opts.Stdout, "\n")
	_, _ = fmt.Fprintf(opts.Stdout, "Uninstall:\n")
	_, _ = fmt.Fprintf(opts.Stdout, "  %s\n", ui.Paint(opts.Stdout, ui.ColorInfo, "rg hook uninstall"))

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
		_, _ = fmt.Fprintf(opts.Stdout, "%s No RegressGuard hook block found in %s\n", ui.Paint(opts.Stdout, ui.ColorInfo, "i"), hookPath)
		return nil
	}

	updated := removeBlock(existing)

	// If nothing meaningful remains, delete the file.
	if isEmptyHook(updated) {
		if err := os.Remove(hookPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove hook file: %w", err)
		}
		_, _ = fmt.Fprintf(opts.Stdout, "%s Removed pre-commit hook\n", ui.Paint(opts.Stdout, ui.ColorOK, "OK"))
		_, _ = fmt.Fprintf(opts.Stdout, "   %s deleted (was empty after removal)\n", ui.Paint(opts.Stdout, ui.ColorMuted, hookPath))
		return nil
	}

	if err := os.WriteFile(hookPath, []byte(updated), 0o755); err != nil {
		return fmt.Errorf("write pre-commit hook: %w", err)
	}

	_, _ = fmt.Fprintf(opts.Stdout, "%s Removed RegressGuard block from %s\n", ui.Paint(opts.Stdout, ui.ColorOK, "OK"), hookPath)
	return nil
}

// managedBlock returns the full text of the RegressGuard-managed block.
// The binary path is resolved at install time to avoid ripgrep conflicts.
func managedBlock() string {
	binPath := resolveBinaryPath()
	script := strings.ReplaceAll(hookScript, "{{BIN}}", binPath)
	return blockBegin + "\n" + script + "\n" + blockEnd + "\n"
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
