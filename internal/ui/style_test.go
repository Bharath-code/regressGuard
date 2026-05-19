package ui

import (
	"bytes"
	"os"
	"testing"
)

func TestColorEnabledRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORCE_COLOR", "1")

	if ColorEnabled(os.Stdout) {
		t.Fatal("NO_COLOR must disable ANSI output even when FORCE_COLOR is set")
	}
}

func TestColorEnabledRespectsDumbTerminal(t *testing.T) {
	t.Setenv("TERM", "dumb")

	if ColorEnabled(os.Stdout) {
		t.Fatal("TERM=dumb must disable ANSI output")
	}
}

func TestColorEnabledUsesForceColor(t *testing.T) {
	unsetEnv(t, "NO_COLOR")
	unsetEnv(t, "TERM")
	t.Setenv("FORCE_COLOR", "1")

	if !ColorEnabled(bytes.NewBuffer(nil)) {
		t.Fatal("FORCE_COLOR must enable ANSI output for non-file writers")
	}
}

func TestPaintOmitsANSINonTTY(t *testing.T) {
	unsetEnv(t, "NO_COLOR")
	unsetEnv(t, "FORCE_COLOR")
	unsetEnv(t, "TERM")
	var out bytes.Buffer
	got := Paint(&out, ColorFail, "CRITICAL")
	if got != "CRITICAL" {
		t.Fatalf("non-TTY output should not be colored, got %q", got)
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	original, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(key, original)
			return
		}
		_ = os.Unsetenv(key)
	})
}
