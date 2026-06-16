package ui

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Banner styles for the final verdict display.
var (
	// passBanner is a green-bordered box for clean checks.
	passBannerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2DA44E")).
			Padding(0, 2).
			Bold(true)

	// criticalBannerStyle is a red-bordered box for regressions.
	criticalBannerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#CF222E")).
				Padding(0, 2).
				Bold(true)

	// warningBannerStyle is a yellow-bordered box for warnings.
	warningBannerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#B88700")).
				Padding(0, 2).
				Bold(true)

	// headerStyle for section headers.
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

	// successCheckStyle for the celebration checkmark.
	successCheckStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#2DA44E")).
				Bold(true)

	// failXStyle for the critical X.
	failXStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CF222E")).
			Bold(true)
)

// PassBanner renders a styled green-bordered pass verdict.
// Only renders the styled version on TTY; returns plain text otherwise.
func PassBanner(w io.Writer, text string) string {
	if !ColorEnabled(w) {
		return text
	}
	return passBannerStyle.Render(text)
}

// CriticalBanner renders a styled red-bordered critical verdict.
func CriticalBanner(w io.Writer, text string) string {
	if !ColorEnabled(w) {
		return text
	}
	return criticalBannerStyle.Render(text)
}

// WarningBanner renders a styled yellow-bordered warning verdict.
func WarningBanner(w io.Writer, text string) string {
	if !ColorEnabled(w) {
		return text
	}
	return warningBannerStyle.Render(text)
}

// SuccessCelebration prints an animated success celebration on TTY.
// Shows a brief "sparkle" effect around the checkmark.
func SuccessCelebration(w io.Writer, message string) {
	if !ColorEnabled(w) {
		_, _ = fmt.Fprintln(w, SymbolPass+" "+message)
		return
	}

	// Calm default: render the final styled line instantly, no frames.
	if !animationsEnabled {
		_, _ = fmt.Fprintln(w, successCheckStyle.Render("OK")+" "+message)
		return
	}

	// Frame 1: dim
	frame1 := Paint(w, ColorMuted, ".") + " " + message
	_, _ = fmt.Fprint(w, "\r"+frame1)
	time.Sleep(60 * time.Millisecond)

	// Frame 2: growing
	frame2 := Paint(w, ColorInfo, "*") + " " + message
	_, _ = fmt.Fprint(w, "\r\033[K"+frame2)
	time.Sleep(60 * time.Millisecond)

	// Frame 3: full checkmark with color
	check := successCheckStyle.Render("OK")
	frame3 := check + " " + message
	_, _ = fmt.Fprint(w, "\r\033[K"+frame3+"\n")
}

// CriticalReveal prints the critical verdict with a brief dramatic pause.
func CriticalReveal(w io.Writer, message string) {
	if !ColorEnabled(w) {
		_, _ = fmt.Fprintln(w, SymbolCritical+" "+message)
		return
	}

	// Brief pause before revealing bad news (only when animating).
	if animationsEnabled {
		time.Sleep(100 * time.Millisecond)
	}

	x := failXStyle.Render("X")
	_, _ = fmt.Fprintln(w, x+" "+message)
}

// AnimatedTableRow prints a table row with a brief slide-in effect on TTY.
func AnimatedTableRow(w io.Writer, row string, delay time.Duration) {
	if animationsEnabled && ColorEnabled(w) {
		time.Sleep(delay)
	}
	_, _ = fmt.Fprintln(w, row)
}
