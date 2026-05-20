// Package ui provides terminal micro-interactions for RegressGuard.
// The Spinner type provides animated phase indicators styled with Lip Gloss,
// respects TTY/NO_COLOR settings, and auto-disables in non-interactive modes.
package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Spinner frame sets — matching charmbracelet/bubbles spinner styles.
var (
	// MiniDot is the default spinner — same as bubbles/spinner.MiniDot.
	miniDotFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	// Dot is a larger braille spinner — same as bubbles/spinner.Dot.
	dotFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

	// Pulse is a block-based spinner.
	pulseFrames = []string{"█", "▓", "▒", "░"}
)

// spinnerFrames is the active frame set (exported for routeprogress.go).
var spinnerFrames = miniDotFrames

// Lip Gloss styles for spinner components.
var (
	spinnerCharStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0969DA")).
				Bold(true)

	spinnerMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#383838", Dark: "#CCCCCC"})

	spinnerTimerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6E7781"))

	spinnerSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#2DA44E")).
				Bold(true)

	spinnerFailStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CF222E")).
				Bold(true)

	spinnerWarnStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#B88700")).
				Bold(true)
)

const (
	spinnerInterval = 80 * time.Millisecond
	// staggerDelay is the pause between result lines for the "drop into place" feel.
	staggerDelay = 80 * time.Millisecond
)

// Spinner renders an animated phase indicator on a TTY stderr stream.
// Uses Lip Gloss for styling. Gracefully degrades to no output on non-TTY.
type Spinner struct {
	w       io.Writer
	message string
	active  bool
	done    chan struct{}
	mu      sync.Mutex
	elapsed time.Duration
	start   time.Time
	enabled bool
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
	frame := 0
	ticker := time.NewTicker(spinnerInterval)
	defer ticker.Stop()

	// Render first frame immediately.
	s.renderFrame(frame)

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			frame = (frame + 1) % len(spinnerFrames)
			s.renderFrame(frame)
		}
	}
}

func (s *Spinner) renderFrame(frame int) {
	s.mu.Lock()
	elapsed := time.Since(s.start)
	msg := s.message
	s.mu.Unlock()

	// Style the spinner character with Lip Gloss.
	spinChar := spinnerCharStyle.Render(spinnerFrames[frame])
	styledMsg := spinnerMsgStyle.Render(msg)

	var line string
	if elapsed > 1*time.Second {
		// Show elapsed timer for long operations.
		timerStr := spinnerTimerStyle.Render(fmtElapsed(elapsed))
		line = fmt.Sprintf("\r%s %s %s", spinChar, styledMsg, timerStr)
	} else {
		line = fmt.Sprintf("\r%s %s", spinChar, styledMsg)
	}

	// Pad to clear any previous longer line.
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
