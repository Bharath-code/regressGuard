package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Premium CLI components for world-class output.
// Every command uses these for consistent Stripe-level presentation.

var (
	// headerBarStyle is the branded header shown at the top of every command.
	headerBarStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#1F2328", Dark: "#E6EDF3"})

	// separatorStyle is for dim horizontal rules between sections.
	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#30363D"))

	// footerStyle is for the subtle version/timing line at the bottom.
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6E7781"))

	// timingStyle is for elapsed time display.
	timingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6E7781"))

	// tableHeaderStyle is for column headers in data tables.
	tableHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8B949E")).
				Bold(true)

	// tableRowStyle is for data rows.
	tableRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1F2328", Dark: "#E6EDF3"})
)

// Header renders the branded command header bar.
// Example: "RegressGuard  check" or "RegressGuard  snapshot"
func Header(w io.Writer, command string) string {
	if !ColorEnabled(w) {
		return command
	}
	brand := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#1F2328", Dark: "#E6EDF3"}).
		Render("RegressGuard")
	sep := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#30363D")).
		Render("  ")
	cmd := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#0969DA")).
		Bold(true).
		Render(command)
	return brand + sep + cmd
}

// Separator renders a dim horizontal line.
// Width adapts to MaxWidth.
func Separator(w io.Writer) string {
	line := strings.Repeat("─", 50)
	if !ColorEnabled(w) {
		return line
	}
	return separatorStyle.Render(line)
}

// SeparatorLight renders a shorter, lighter separator.
func SeparatorLight(w io.Writer) string {
	line := strings.Repeat("─", 36)
	if !ColorEnabled(w) {
		return line
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#21262D")).
		Render(line)
}

// Footer renders the subtle timing + version footer.
// Example: "Done in 1.8s"
func Footer(w io.Writer, elapsed time.Duration) string {
	if !ColorEnabled(w) {
		return fmt.Sprintf("Done in %s", formatDuration(elapsed))
	}
	return footerStyle.Render(fmt.Sprintf("Done in %s", formatDuration(elapsed)))
}

// FooterWithVersion renders footer with version info.
func FooterWithVersion(w io.Writer, elapsed time.Duration, version string) string {
	if !ColorEnabled(w) {
		return fmt.Sprintf("Done in %s  rg %s", formatDuration(elapsed), version)
	}
	timing := timingStyle.Render(fmt.Sprintf("Done in %s", formatDuration(elapsed)))
	ver := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#30363D")).
		Render("  rg " + version)
	return timing + ver
}

// TableHeaderRow renders a styled table header.
func TableHeaderRow(w io.Writer, columns ...string) string {
	row := strings.Join(columns, "")
	if !ColorEnabled(w) {
		return row
	}
	return tableHeaderStyle.Render(row)
}

// TableDataRow renders a styled data row.
func TableDataRow(w io.Writer, content string) string {
	if !ColorEnabled(w) {
		return content
	}
	return tableRowStyle.Render(content)
}

// ResultLine renders a status result line with consistent formatting.
// symbol: OK/!/X, label: "Tests"/"Routes", detail: "4 passed, 0 failed"
func ResultLine(w io.Writer, status string, label, detail string) string {
	var sym string
	if ColorEnabled(w) {
		switch status {
		case "pass":
			sym = StatusPass.Render(SymbolPass)
		case "warn":
			sym = StatusWarn.Render(SymbolWarning)
		case "fail":
			sym = StatusFail.Render(SymbolCritical)
		case "info":
			sym = StatusInfo.Render(SymbolInfo)
		case "skip":
			sym = Muted.Render(SymbolSkipped)
		default:
			sym = SymbolPass
		}
	} else {
		switch status {
		case "pass":
			sym = SymbolPass
		case "warn":
			sym = SymbolWarning
		case "fail":
			sym = SymbolCritical
		case "info":
			sym = SymbolInfo
		case "skip":
			sym = SymbolSkipped
		default:
			sym = SymbolPass
		}
	}

	formattedLabel := fmt.Sprintf("%-10s", label)
	if ColorEnabled(w) {
		return fmt.Sprintf("%s %s %s", sym, formattedLabel, detail)
	}
	return fmt.Sprintf("%s %s %s", sym, formattedLabel, detail)
}

// NextSection renders the "Next:" section with styled commands.
func NextSection(w io.Writer, commands ...string) []string {
	lines := []string{""}
	if ColorEnabled(w) {
		lines = append(lines, NextLabel.Render("Next:"))
	} else {
		lines = append(lines, "Next:")
	}
	for _, cmd := range commands {
		if ColorEnabled(w) {
			lines = append(lines, "  "+NextCommand.Render(cmd))
		} else {
			lines = append(lines, "  "+cmd)
		}
	}
	return lines
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
