package ui

import "github.com/charmbracelet/lipgloss"

// RegressGuard Premium Mono Theme
// Inspired by Stripe CLI, Vercel CLI, and Charm tools.
// Designed to feel like a $1B infrastructure tool.

// Brand colors — semantic, works on dark and light terminals.
var (
	// Primary brand color — used for active elements, links, commands.
	BrandBlue = lipgloss.Color("#0969DA")

	// Success — pass, installed, completed.
	BrandGreen = lipgloss.Color("#2DA44E")

	// Warning — non-blocking, timing, skipped.
	BrandYellow = lipgloss.Color("#B88700")

	// Critical — regression, blocked, error.
	BrandRed = lipgloss.Color("#CF222E")

	// Muted — metadata, paths, secondary info.
	BrandMuted = lipgloss.Color("#6E7781")

	// Border — tables, separators, boxes.
	BrandBorder = lipgloss.Color("#8C959F")

	// Text — primary foreground, adapts to terminal theme.
	BrandText = lipgloss.AdaptiveColor{Light: "#1F2328", Dark: "#E6EDF3"}

	// Subtle text — secondary foreground.
	BrandSubtle = lipgloss.AdaptiveColor{Light: "#656D76", Dark: "#8B949E"}
)

// Typography styles.
var (
	// Title is for section headers (bold, primary text).
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(BrandText)

	// Subtitle is for secondary headers.
	Subtitle = lipgloss.NewStyle().
			Foreground(BrandSubtle)

	// Body is for regular text.
	Body = lipgloss.NewStyle().
		Foreground(BrandText)

	// Code is for commands, paths, and code snippets.
	Code = lipgloss.NewStyle().
		Foreground(BrandBlue)

	// Muted is for metadata and secondary information.
	Muted = lipgloss.NewStyle().
		Foreground(BrandMuted)

	// Bold is for emphasis.
	Bold = lipgloss.NewStyle().
		Bold(true).
		Foreground(BrandText)
)

// Status styles.
var (
	// StatusPass is for pass/success indicators.
	StatusPass = lipgloss.NewStyle().
			Foreground(BrandGreen).
			Bold(true)

	// StatusWarn is for warning indicators.
	StatusWarn = lipgloss.NewStyle().
			Foreground(BrandYellow).
			Bold(true)

	// StatusFail is for critical/error indicators.
	StatusFail = lipgloss.NewStyle().
			Foreground(BrandRed).
			Bold(true)

	// StatusInfo is for informational indicators.
	StatusInfo = lipgloss.NewStyle().
			Foreground(BrandBlue).
			Bold(true)
)

// Box styles — for verdict banners.
var (
	// PassBox is a green-bordered rounded box for pass verdicts.
	PassBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BrandGreen).
		Padding(0, 2).
		Bold(true).
		Foreground(BrandGreen)

	// CriticalBox is a red-bordered rounded box for critical verdicts.
	CriticalBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BrandRed).
			Padding(0, 2).
			Bold(true).
			Foreground(BrandRed)

	// WarningBox is a yellow-bordered rounded box for warning verdicts.
	WarningBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BrandYellow).
			Padding(0, 2).
			Bold(true).
			Foreground(BrandYellow)

	// InfoBox is a blue-bordered rounded box for informational messages.
	InfoBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BrandBlue).
		Padding(0, 2).
		Foreground(BrandBlue)
)

// Table styles.
var (
	// TableHeader is for table column headers.
	TableHeader = lipgloss.NewStyle().
			Foreground(BrandMuted).
			Bold(true)

	// TableRow is for regular table rows.
	TableRow = lipgloss.NewStyle().
			Foreground(BrandText)

	// TableSeparator is for horizontal rules in tables.
	TableSeparator = lipgloss.NewStyle().
			Foreground(BrandBorder)
)

// Next step styles.
var (
	// NextCommand is for suggested commands in "Next:" sections.
	NextCommand = lipgloss.NewStyle().
			Foreground(BrandBlue)

	// NextLabel is for the "Next:" label itself.
	NextLabel = lipgloss.NewStyle().
			Foreground(BrandSubtle).
			Bold(true)
)
