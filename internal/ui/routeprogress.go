package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Lip Gloss styles for route progress display.
var (
	routeMethodStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6E7781")).
				Width(6)

	routePathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#383838", Dark: "#CCCCCC"}).
			Width(30)

	routePendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6E7781"))

	routeDoneCodeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#2DA44E")).
				Bold(true)

	routeDoneMSStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6E7781"))

	routeFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CF222E")).
				Bold(true)

	routeSkippedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6E7781")).
				Italic(true)

	progressFilledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#2DA44E"))

	progressEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6E7781"))

	progressCountStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6E7781"))
)

// RouteProgress displays a live-updating route status table on TTY.
// Each route shows its current state (spinning/done/failed/skipped).
// On non-TTY, it produces no output (the final result is shown by the caller).
type RouteProgress struct {
	w       io.Writer
	routes  []routeState
	mu      sync.Mutex
	active  bool
	done    chan struct{}
	enabled bool
	frame   int
}

type routeState struct {
	Method string
	Path   string
	Status string // "pending", "done", "failed", "skipped"
	Code   int
	MS     int64
}

// NewRouteProgress creates a live route progress display.
// Only animates on TTY with color enabled.
func NewRouteProgress(w io.Writer, routes []struct{ Method, Path string }) *RouteProgress {
	states := make([]routeState, len(routes))
	for i, r := range routes {
		states[i] = routeState{Method: r.Method, Path: r.Path, Status: "pending"}
	}
	return &RouteProgress{
		w:       w,
		routes:  states,
		enabled: ColorEnabled(w),
		done:    make(chan struct{}),
	}
}

// Start begins the live display animation.
func (rp *RouteProgress) Start() {
	if !rp.enabled || len(rp.routes) == 0 {
		return
	}
	rp.mu.Lock()
	rp.active = true
	rp.mu.Unlock()

	go rp.run()
}

// MarkDone marks a route as completed.
func (rp *RouteProgress) MarkDone(index int, code int, ms int64) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if index >= 0 && index < len(rp.routes) {
		rp.routes[index].Status = "done"
		rp.routes[index].Code = code
		rp.routes[index].MS = ms
	}
}

// MarkFailed marks a route as failed.
func (rp *RouteProgress) MarkFailed(index int) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if index >= 0 && index < len(rp.routes) {
		rp.routes[index].Status = "failed"
	}
}

// MarkSkipped marks a route as skipped.
func (rp *RouteProgress) MarkSkipped(index int) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if index >= 0 && index < len(rp.routes) {
		rp.routes[index].Status = "skipped"
	}
}

// Stop halts the live display and clears the rendered lines.
func (rp *RouteProgress) Stop() {
	rp.mu.Lock()
	if !rp.active {
		rp.mu.Unlock()
		return
	}
	rp.active = false
	rp.mu.Unlock()

	close(rp.done)

	// Clear all rendered lines.
	if rp.enabled {
		rp.clearDisplay()
	}
}

// CompletedCount returns how many routes are done/failed/skipped.
func (rp *RouteProgress) CompletedCount() int {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	count := 0
	for _, r := range rp.routes {
		if r.Status != "pending" {
			count++
		}
	}
	return count
}

func (rp *RouteProgress) run() {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	rp.render()

	for {
		select {
		case <-rp.done:
			return
		case <-ticker.C:
			rp.frame++
			rp.render()
		}
	}
}

func (rp *RouteProgress) render() {
	rp.mu.Lock()
	routes := make([]routeState, len(rp.routes))
	copy(routes, rp.routes)
	frame := rp.frame
	rp.mu.Unlock()

	// Move cursor up to overwrite previous render.
	if frame > 0 {
		_, _ = fmt.Fprintf(rp.w, "\033[%dA", len(routes)+1)
	}

	// Progress bar at top with Lip Gloss styling.
	completed := 0
	for _, r := range routes {
		if r.Status != "pending" {
			completed++
		}
	}
	bar := renderProgressBar(completed, len(routes), 20)
	spinChar := spinnerCharStyle.Render(spinnerFrames[frame%len(spinnerFrames)])
	_, _ = fmt.Fprintf(rp.w, "\r\033[K  %s Routes %s\n", spinChar, bar)

	// Route lines with Lip Gloss styling.
	for _, r := range routes {
		method := routeMethodStyle.Render(r.Method)
		path := routePathStyle.Render(truncatePath(r.Path, 30))

		var status string
		switch r.Status {
		case "pending":
			pendingSpinner := spinnerCharStyle.Render(spinnerFrames[frame%len(spinnerFrames)])
			status = pendingSpinner
		case "done":
			codeStr := routeDoneCodeStyle.Render(fmt.Sprintf("%d", r.Code))
			msStr := routeDoneMSStyle.Render(fmt.Sprintf("%dms", r.MS))
			status = codeStr + " " + msStr
		case "failed":
			status = routeFailedStyle.Render("ERR")
		case "skipped":
			status = routeSkippedStyle.Render("skip")
		}

		_, _ = fmt.Fprintf(rp.w, "\033[K    %s %s %s\n", method, path, status)
	}
}

func (rp *RouteProgress) clearDisplay() {
	// Move up and clear each line.
	n := len(rp.routes) + 1
	for i := 0; i < n; i++ {
		_, _ = fmt.Fprint(rp.w, "\033[A\033[K")
	}
}

// renderProgressBar creates a Lip Gloss styled progress bar.
func renderProgressBar(completed, total, width int) string {
	if total == 0 {
		return ""
	}
	pct := float64(completed) / float64(total)
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}

	filledStr := progressFilledStyle.Render(strings.Repeat("█", filled))
	emptyStr := progressEmptyStyle.Render(strings.Repeat("░", width-filled))
	countStr := progressCountStyle.Render(fmt.Sprintf("%d/%d", completed, total))

	return filledStr + emptyStr + " " + countStr
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

// ProgressBar renders a simple progress bar (for external use).
func ProgressBar(w io.Writer, completed, total int, width int) string {
	if total == 0 {
		return ""
	}
	if !ColorEnabled(w) {
		pct := float64(completed) / float64(total)
		filled := int(pct * float64(width))
		if filled > width {
			filled = width
		}
		bar := strings.Repeat("#", filled) + strings.Repeat("-", width-filled)
		return fmt.Sprintf("[%s] %d/%d", bar, completed, total)
	}
	return renderProgressBar(completed, total, width)
}
