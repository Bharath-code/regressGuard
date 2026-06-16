package ui

import (
	"io"
	"os"
)

const (
	MaxWidth = 80

	SymbolPass     = "OK"
	SymbolWarning  = "!"
	SymbolCritical = "X"
	SymbolInfo     = "i"
	SymbolSkipped  = "-"
	SymbolRunning  = ">"
)

const (
	ansiReset = "\x1b[0m"

	ansiOK      = "\x1b[32m"
	ansiWarn    = "\x1b[33m"
	ansiFail    = "\x1b[31m"
	ansiInfo    = "\x1b[36m"
	ansiMuted   = "\x1b[90m"
	ansiBold    = "\x1b[1m"
	ansiBorder  = "\x1b[2m"
	ansiDefault = ""
)

type Color string

const (
	ColorDefault Color = ansiDefault
	ColorOK      Color = ansiOK
	ColorWarn    Color = ansiWarn
	ColorFail    Color = ansiFail
	ColorInfo    Color = ansiInfo
	ColorMuted   Color = ansiMuted
	ColorBold    Color = ansiBold
	ColorBorder  Color = ansiBorder
)

// animationsEnabled gates optional motion effects: staggered reveal, slide-in
// table rows, success/critical reveals, and the live route-progress table. Off
// by default so rg renders instantly; opt in with `rg check --celebrate`.
var animationsEnabled bool

// SetAnimations toggles optional motion effects. Call once at startup before
// any rendering. Has no effect on non-TTY/JSON/hook output (already motionless).
func SetAnimations(on bool) { animationsEnabled = on }

// AnimationsEnabled reports whether optional motion effects are active.
func AnimationsEnabled() bool { return animationsEnabled }

func ColorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

func IsTerminal(stream any) bool {
	file, ok := stream.(*os.File)
	if !ok {
		return false
	}
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

func Paint(w io.Writer, color Color, text string) string {
	if color == ColorDefault || !ColorEnabled(w) {
		return text
	}
	return string(color) + text + ansiReset
}
