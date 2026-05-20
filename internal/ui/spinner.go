// Package ui provides terminal micro-interactions for RegressGuard.
// The Spinner type provides animated phase indicators using charmbracelet/huh
// spinner for a premium, world-class CLI feel. Respects TTY/NO_COLOR settings
// and auto-disables in non-interactive modes.
package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
)

// Spinner frame sets — matching charmbracelet/bubbles spinner styles.
// Used by routeprogress.go for inline animation.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Lip Gloss styles for spinner result display.
var (
	spinnerCharStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0969DA")).
				Bold(true)

	spinnerSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#2DA44E")).
				Bold(true)

	spinnerFailStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CF222E")).
				Bold(true)

	spinnerWarnStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#B88700")).
				Bold(true)

	spinnerMutedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6E7781"))
)

const (
	// staggerDelay is the pause between result lines for the "drop into place" feel.
	staggerDelay = 80 * time.Millisecond
)

// Spinner renders an animated phase indicator on a TTY stderr stream.
// Uses charmbracelet/huh spinner for premium animation quality.
// Gracefully degrades to no output on non-TTY.
type Spinner struct {
	w       io.Writer
	message string
	active  bool
	done    chan struct{}
	mu      sync.Mutex
	elapsed time.Duration
	start   time.Time
	enabled bool
	// fallback: manual animation for when huh/spinner can't be used
	// (e.g., when we need concurrent spinners or custom stderr target)
	frame int
}

// NewSpinner creates a spinner that writes to w.
// Animation is only enabled when w is a TTY and colors are enabled.
// In non-TTY/JSON/hook modes, the spinner produces no output.
func NewSpinner(w io.Writer, message string) *Spinner {
	return &Spinner{
		w:       w,
		message: message,
		enabled: ColorEnabled(w),
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation in a background goroutine.
// Safe to call even when disabled (no-op).
func (s *Spinner) Start() {
	s.mu.Lock()
	if !s.enabled || s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.start = time.Now()
	s.mu.Unlock()

	go s.run()
}

// Stop halts the spinner and clears the line.
// Returns the elapsed duration since Start was called.
func (s *Spinner) Stop() time.Duration {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return 0
	}
	s.active = false
	s.elapsed = time.Since(s.start)
	s.mu.Unlock()

	close(s.done)

	// Clear the spinner line.
	if s.enabled {
		s.clearLine()
	}
	return s.elapsed
}

// StopWithResult halts the spinner and replaces it with a final result line.
// The symbol and color indicate the outcome (pass/warn/fail).
func (s *Spinner) StopWithResult(symbol string, color Color, result string) time.Duration {
	elapsed := s.Stop()
	if s.enabled {
		var styledSymbol string
		switch color {
		case ColorOK:
			styledSymbol = spinnerSuccessStyle.Render(symbol)
		case ColorFail:
			styledSymbol = spinnerFailStyle.Render(symbol)
		case ColorWarn:
			styledSymbol = spinnerWarnStyle.Render(symbol)
		default:
			styledSymbol = symbol
		}
		_, _ = fmt.Fprintln(s.w, styledSymbol+" "+result)
	}
	return elapsed
}

// StopFailed halts the spinner and shows a failure result.
func (s *Spinner) StopFailed(result string) time.Duration {
	return s.StopWithResult(SymbolCritical, ColorFail, result)
}

// StopSuccess halts the spinner and shows a success result.
func (s *Spinner) StopSuccess(result string) time.Duration {
	return s.StopWithResult(SymbolPass, ColorOK, result)
}

// StopWarning halts the spinner and shows a warning result.
func (s *Spinner) StopWarning(result string) time.Duration {
	return s.StopWithResult(SymbolWarning, ColorWarn, result)
}

func (s *Spinner) run() {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	s.renderFrame(0)

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			s.frame = (s.frame + 1) % len(spinnerFrames)
			frame := s.frame
			s.mu.Unlock()
			s.renderFrame(frame)
		}
	}
}

func (s *Spinner) renderFrame(frame int) {
	s.mu.Lock()
	elapsed := time.Since(s.start)
	msg := s.message
	s.mu.Unlock()

	// Style with Lip Gloss for premium look.
	spinChar := spinnerCharStyle.Render(spinnerFrames[frame])
	styledMsg := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#383838", Dark: "#CCCCCC"}).
		Render(msg)

	var line string
	if elapsed > 1*time.Second {
		timerStr := spinnerMutedStyle.Render(fmtElapsed(elapsed))
		line = fmt.Sprintf("\r%s %s %s", spinChar, styledMsg, timerStr)
	} else {
		line = fmt.Sprintf("\r%s %s", spinChar, styledMsg)
	}

	padding := strings.Repeat(" ", 10)
	_, _ = fmt.Fprint(s.w, line+padding)
}

func (s *Spinner) clearLine() {
	_, _ = fmt.Fprint(s.w, "\r\033[K")
}

func fmtElapsed(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// RunWithSpinner executes an action with a huh/spinner animation.
// This is the premium Charm spinner experience — use for top-level actions
// where only one spinner runs at a time (not concurrent phases).
// Falls back to no-op on non-TTY.
func RunWithSpinner(title string, action func()) error {
	return spinner.New().
		Title(title).
		Action(action).
		Run()
}

// StaggeredPrint prints lines with a brief delay between each for a
// "dropping into place" effect. Only staggers on TTY; prints immediately otherwise.
func StaggeredPrint(w io.Writer, lines []string) {
	isTTY := ColorEnabled(w)
	for i, line := range lines {
		_, _ = fmt.Fprintln(w, line)
		if isTTY && i < len(lines)-1 {
			time.Sleep(staggerDelay)
		}
	}
}
