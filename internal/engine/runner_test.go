package engine

import (
	"testing"
)

func TestParseTestOutput_vitest(t *testing.T) {
	output := `
 ✓ src/auth.test.ts (3)
 ✓ src/user.test.ts (5)

 Test Files  2 passed (2)
 Tests  8 passed (8)
 Duration  1.23s
`
	result := parseTestOutput(output)
	if result.Passed != 8 {
		t.Errorf("expected 8 passed, got %d", result.Passed)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}
}

func TestParseTestOutput_vitestWithFailures(t *testing.T) {
	output := `
 ✓ src/auth.test.ts (3)
 ✗ src/user.test.ts (5)

 Test Files  1 failed, 1 passed (2)
 Tests  2 failed | 6 passed (8)
 Duration  1.23s
`
	result := parseTestOutput(output)
	if result.Passed < 6 {
		t.Errorf("expected at least 6 passed, got %d", result.Passed)
	}
	if result.Failed < 2 {
		t.Errorf("expected at least 2 failed, got %d", result.Failed)
	}
}

func TestParseTestOutput_jest(t *testing.T) {
	output := `
PASS src/auth.test.js
PASS src/user.test.js

Tests: 1 skipped, 0 failed, 42 passed, 43 total
Test Suites: 2 passed, 2 total
`
	result := parseTestOutput(output)
	if result.Passed != 42 {
		t.Errorf("expected 42 passed, got %d", result.Passed)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
}

func TestParseTestOutput_jestWithFailures(t *testing.T) {
	output := `
FAIL src/auth.test.js

Tests: 3 failed, 39 passed, 42 total
`
	result := parseTestOutput(output)
	if result.Passed != 39 {
		t.Errorf("expected 39 passed, got %d", result.Passed)
	}
	if result.Failed != 3 {
		t.Errorf("expected 3 failed, got %d", result.Failed)
	}
}

func TestParseTestOutput_empty(t *testing.T) {
	result := parseTestOutput("")
	if result.Passed != 0 || result.Failed != 0 {
		t.Errorf("expected 0/0 for empty output, got %d/%d", result.Passed, result.Failed)
	}
}

func TestSplitCommand_simple(t *testing.T) {
	parts := splitCommand("npm test")
	if len(parts) != 2 || parts[0] != "npm" || parts[1] != "test" {
		t.Errorf("unexpected split: %v", parts)
	}
}

func TestSplitCommand_quoted(t *testing.T) {
	// Quoted strings with spaces are kept as a single token.
	// "npx vitest --reporter verbose mode" → 4 parts: npx, vitest, --reporter, "verbose mode"
	parts := splitCommand(`npx vitest --reporter "verbose mode"`)
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d: %v", len(parts), parts)
	}
	if parts[3] != "verbose mode" {
		t.Errorf("expected 'verbose mode' as single token, got %q", parts[3])
	}
}

func TestSplitCommand_empty(t *testing.T) {
	parts := splitCommand("")
	if len(parts) != 0 {
		t.Errorf("expected empty slice, got %v", parts)
	}
}
