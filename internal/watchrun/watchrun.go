// Package watchrun implements the rg watch command.
// It watches project files for changes and automatically runs rg check
// when modifications are detected, providing continuous regression feedback.
package watchrun

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Bharath-code/regressguard/internal/checkrun"
	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/failures"
	"github.com/Bharath-code/regressguard/internal/ui"
	"github.com/fsnotify/fsnotify"
)

// Options configures a watch run.
type Options struct {
	ProjectRoot string
	Debounce    time.Duration // minimum time between checks (default: 2s)
	Stdout      io.Writer
	Stderr      io.Writer
}

// Run starts the file watcher and blocks until interrupted.
func Run(opts Options) error {
	opts = withDefaults(opts)

	// Validate config exists.
	if !config.Exists(opts.ProjectRoot) {
		return failures.Actionable{
			Title:       "rg watch failed: no config found.",
			Cause:       "RegressGuard has not been initialized for this project.",
			Next:        "rg init",
			MoreContext: "rg watch --help",
		}
	}

	cfg, err := config.Load(opts.ProjectRoot)
	if err != nil {
		return failures.Actionable{
			Title:       "rg watch failed: config is invalid.",
			Cause:       err.Error(),
			Next:        "rg init --yes",
			MoreContext: "rg watch --help",
		}
	}

	// Create file watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	// Add source directories to watch.
	dirs := discoverWatchDirs(opts.ProjectRoot, cfg.Framework)
	for _, dir := range dirs {
		_ = watcher.Add(dir)
	}

	// Print startup message.
	_, _ = fmt.Fprintln(opts.Stdout, ui.Header(opts.Stdout, "watch"))
	_, _ = fmt.Fprintln(opts.Stdout, ui.Separator(opts.Stdout))
	_, _ = fmt.Fprintln(opts.Stdout)
	_, _ = fmt.Fprintf(opts.Stdout, "%s Watching %d directories for changes...\n",
		paint(opts.Stdout, ui.ColorOK, ui.SymbolPass), len(dirs))
	_, _ = fmt.Fprintf(opts.Stdout, "%s Press Ctrl+C to stop.\n",
		paint(opts.Stdout, ui.ColorMuted, ui.SymbolInfo))
	_, _ = fmt.Fprintln(opts.Stdout)

	// Debounce state.
	var (
		mu         sync.Mutex
		lastCheck  time.Time
		checkTimer *time.Timer
	)

	// Run check with debouncing.
	triggerCheck := func(changedFile string) {
		mu.Lock()
		defer mu.Unlock()

		// Skip if we just ran a check.
		if time.Since(lastCheck) < opts.Debounce {
			if checkTimer == nil {
				checkTimer = time.AfterFunc(opts.Debounce-time.Since(lastCheck), func() {
					runCheck(opts, "")
					mu.Lock()
					lastCheck = time.Now()
					checkTimer = nil
					mu.Unlock()
				})
			}
			return
		}

		lastCheck = time.Now()
		go runCheck(opts, changedFile)
	}

	// Handle signals for clean shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Event loop.
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// Only trigger on writes/creates/renames (not chmod).
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				if shouldWatch(event.Name) {
					triggerCheck(event.Name)
				}
				// If a new directory was created, watch it too.
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						_ = watcher.Add(event.Name)
					}
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			_, _ = fmt.Fprintf(opts.Stderr, "%s Watcher error: %v\n", ui.SymbolWarning, err)

		case <-sigCh:
			_, _ = fmt.Fprintln(opts.Stdout)
			_, _ = fmt.Fprintf(opts.Stdout, "%s Watch stopped.\n",
				paint(opts.Stdout, ui.ColorMuted, ui.SymbolInfo))
			return nil
		}
	}
}

// runCheck executes rg check and prints a compact result.
func runCheck(opts Options, changedFile string) {
	_, _ = fmt.Fprintln(opts.Stdout, ui.Separator(opts.Stdout))
	if changedFile != "" {
		rel, _ := filepath.Rel(opts.ProjectRoot, changedFile)
		if rel == "" {
			rel = changedFile
		}
		_, _ = fmt.Fprintf(opts.Stdout, "%s Change detected: %s\n",
			paint(opts.Stdout, ui.ColorInfo, ui.SymbolInfo),
			paint(opts.Stdout, ui.ColorMuted, rel))
	}
	_, _ = fmt.Fprintf(opts.Stdout, "%s Running check...\n",
		paint(opts.Stdout, ui.ColorInfo, ui.SymbolRunning))
	_, _ = fmt.Fprintln(opts.Stdout)

	result, err := checkrun.Run(checkrun.Options{
		ProjectRoot: opts.ProjectRoot,
		Stdout:      opts.Stdout,
		Stderr:      opts.Stderr,
	})

	if err != nil {
		if issue, ok := err.(failures.Actionable); ok {
			_, _ = fmt.Fprintf(opts.Stderr, "%s %s\n",
				paint(opts.Stderr, ui.ColorFail, ui.SymbolCritical), issue.Title)
			_, _ = fmt.Fprintf(opts.Stderr, "  %s\n", issue.Next)
		} else {
			_, _ = fmt.Fprintf(opts.Stderr, "%s %v\n",
				paint(opts.Stderr, ui.ColorFail, ui.SymbolCritical), err)
		}
		_, _ = fmt.Fprintln(opts.Stdout)
		return
	}

	_ = result // output already rendered by checkrun.Run
	_, _ = fmt.Fprintln(opts.Stdout)
}

// discoverWatchDirs finds directories to watch based on the project structure.
func discoverWatchDirs(root, framework string) []string {
	var dirs []string

	// Always watch common source directories.
	candidates := []string{
		"src",
		"app",
		"pages",
		"lib",
		"utils",
		"api",
		"routes",
		"middleware",
		"services",
		"controllers",
	}

	// Framework-specific directories.
	switch framework {
	case "nextjs-app-router":
		candidates = append(candidates, "app/api")
	case "express", "hono":
		candidates = append(candidates, "routes", "src/routes")
	}

	for _, candidate := range candidates {
		dir := filepath.Join(root, candidate)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			// Walk subdirectories too.
			_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					// Skip node_modules, .git, .next, etc.
					base := filepath.Base(path)
					if base == "node_modules" || base == ".git" || base == ".next" ||
						base == "dist" || base == "build" || base == ".regressguard" {
						return filepath.SkipDir
					}
					dirs = append(dirs, path)
				}
				return nil
			})
		}
	}

	// If no standard dirs found, watch the root (but not recursively deep).
	if len(dirs) == 0 {
		dirs = append(dirs, root)
	}

	// Deduplicate.
	seen := make(map[string]bool)
	var unique []string
	for _, d := range dirs {
		if !seen[d] {
			seen[d] = true
			unique = append(unique, d)
		}
	}
	return unique
}

// shouldWatch returns true if the file is a source file worth triggering a check for.
func shouldWatch(path string) bool {
	// Skip hidden files, lock files, and non-source files.
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") {
		return false
	}

	// Skip known non-source files.
	skipExact := map[string]bool{
		"package-lock.json": true,
		"yarn.lock":         true,
		"pnpm-lock.yaml":    true,
		"bun.lockb":         true,
		"node_modules":      true,
	}
	if skipExact[base] {
		return false
	}

	// Only trigger on source file extensions.
	ext := strings.ToLower(filepath.Ext(path))
	sourceExts := map[string]bool{
		".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".mjs": true, ".cjs": true, ".json": true, ".go": true,
		".py": true, ".rs": true, ".vue": true, ".svelte": true,
	}
	return sourceExts[ext]
}

func paint(w io.Writer, color ui.Color, text string) string {
	return ui.Paint(w, color, text)
}

func withDefaults(opts Options) Options {
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = "."
	}
	if opts.Debounce == 0 {
		opts.Debounce = 2 * time.Second
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return opts
}
