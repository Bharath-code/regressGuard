package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// forceColor enables the styled (TTY) path for buffer writers so the
// animation-gating branches are exercised, and resets the animation toggle.
func forceColor(t *testing.T) {
	t.Helper()
	unsetEnv(t, "NO_COLOR")
	unsetEnv(t, "TERM")
	t.Setenv("FORCE_COLOR", "1")
	orig := animationsEnabled
	t.Cleanup(func() { animationsEnabled = orig })
}

func TestSuccessCelebration_animationsOff_rendersInstantly(t *testing.T) {
	forceColor(t)
	SetAnimations(false)

	var out bytes.Buffer
	start := time.Now()
	SuccessCelebration(&out, "Safe to commit.")
	elapsed := time.Since(start)

	if strings.Contains(out.String(), "\r") {
		t.Fatalf("animations off must not emit carriage-return frames, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Safe to commit.") {
		t.Fatalf("message missing from output: %q", out.String())
	}
	if elapsed > 30*time.Millisecond {
		t.Fatalf("animations off must render instantly, took %s", elapsed)
	}
}

func TestSuccessCelebration_animationsOn_animates(t *testing.T) {
	forceColor(t)
	SetAnimations(true)

	var out bytes.Buffer
	SuccessCelebration(&out, "Safe to commit.")

	if !strings.Contains(out.String(), "\r") {
		t.Fatalf("animations on should emit frame updates, got %q", out.String())
	}
}

func TestCriticalReveal_animationsOff_rendersInstantly(t *testing.T) {
	forceColor(t)
	SetAnimations(false)

	var out bytes.Buffer
	start := time.Now()
	CriticalReveal(&out, "Commit blocked.")
	elapsed := time.Since(start)

	if elapsed > 30*time.Millisecond {
		t.Fatalf("animations off must render instantly, took %s", elapsed)
	}
	if !strings.Contains(out.String(), "Commit blocked.") {
		t.Fatalf("message missing from output: %q", out.String())
	}
}

func TestStaggeredPrint_animationsOff_rendersInstantly(t *testing.T) {
	forceColor(t)
	SetAnimations(false)

	lines := []string{"a", "b", "c", "d", "e"}
	var out bytes.Buffer
	start := time.Now()
	StaggeredPrint(&out, lines)
	elapsed := time.Since(start)

	if elapsed > 30*time.Millisecond {
		t.Fatalf("animations off must not stagger, took %s", elapsed)
	}
	if got := strings.Count(out.String(), "\n"); got != len(lines) {
		t.Fatalf("expected %d lines, got %d: %q", len(lines), got, out.String())
	}
}

func TestSpinner_fastOperation_noSpinnerFrames(t *testing.T) {
	forceColor(t)
	SetAnimations(false)

	var out bytes.Buffer
	s := NewSpinner(&out, "Running tests...")
	s.Start()
	// Operation completes well under the >400ms threshold.
	s.StopSuccess("Tests      4 passed, 0 failed")

	for _, frame := range spinnerFrames {
		if strings.Contains(out.String(), frame) {
			t.Fatalf("fast operation must not flash a spinner frame, got %q", out.String())
		}
	}
	if !strings.Contains(out.String(), "4 passed") {
		t.Fatalf("result line missing: %q", out.String())
	}
}
