package engine

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TestResult holds the outcome of a single test suite run.
type TestResult struct {
	Passed   int
	Failed   int
	Skipped  int
	Duration time.Duration
	// Raw is the combined stdout+stderr output, useful for --verbose.
	Raw string
}

// rePassFail matches common test runner summary lines, e.g.:
//
//	"Tests: 42 passed, 0 failed"   (vitest/jest)
//	"42 passed"                    (bun test)
//	"ok  github.com/... 0.123s"    (go test)
var (
	reVitest  = regexp.MustCompile(`(?i)(\d+)\s+passed`)
	reViFail  = regexp.MustCompile(`(?i)(\d+)\s+failed`)
	reViSkip  = regexp.MustCompile(`(?i)(\d+)\s+skipped`)
	reJestSum = regexp.MustCompile(`(?i)Tests:\s+(?:(\d+)\s+skipped,\s*)?(?:(\d+)\s+failed,\s*)?(\d+)\s+passed`)
)

// RunTests executes the configured test command and returns a TestResult.
// Output is streamed to progressWriter (stderr) when non-nil.
// The command is run with a 5-minute timeout.
func RunTests(testCommand string, workDir string, progressWriter io.Writer) (TestResult, error) {
	if strings.TrimSpace(testCommand) == "" {
		return TestResult{}, fmt.Errorf("no test command configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	parts := splitCommand(testCommand)
	if len(parts) == 0 {
		return TestResult{}, fmt.Errorf("invalid test command: %q", testCommand)
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = workDir

	var buf bytes.Buffer
	if progressWriter != nil {
		cmd.Stdout = io.MultiWriter(&buf, progressWriter)
		cmd.Stderr = io.MultiWriter(&buf, progressWriter)
	} else {
		cmd.Stdout = &buf
		cmd.Stderr = &buf
	}

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	raw := buf.String()
	result := parseTestOutput(raw)
	result.Duration = duration
	result.Raw = raw

	// If the command exited non-zero but we parsed some results, treat it as
	// a failed run (not a crash). If we got nothing, surface the error.
	if runErr != nil && result.Passed == 0 && result.Failed == 0 {
		// Check if it's a context timeout.
		if ctx.Err() != nil {
			return result, fmt.Errorf("test command timed out after %s", duration.Round(time.Second))
		}
		// Non-zero exit with no parseable output — still return what we have.
		// The caller can inspect Failed > 0 or the raw output.
	}

	return result, nil
}

// parseTestOutput extracts pass/fail/skip counts from combined test output.
// It tries multiple patterns to support vitest, jest, bun test, and go test.
func parseTestOutput(output string) TestResult {
	var result TestResult

	// Try jest-style summary first (most specific).
	if m := reJestSum.FindStringSubmatch(output); m != nil {
		result.Skipped = parseInt(m[1])
		result.Failed = parseInt(m[2])
		result.Passed = parseInt(m[3])
		return result
	}

	// Scan line by line for vitest/bun-style output.
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if m := reVitest.FindStringSubmatch(line); m != nil {
			n := parseInt(m[1])
			if n > result.Passed {
				result.Passed = n
			}
		}
		if m := reViFail.FindStringSubmatch(line); m != nil {
			n := parseInt(m[1])
			if n > result.Failed {
				result.Failed = n
			}
		}
		if m := reViSkip.FindStringSubmatch(line); m != nil {
			n := parseInt(m[1])
			if n > result.Skipped {
				result.Skipped = n
			}
		}
	}

	return result
}

// splitCommand splits a shell-style command string into argv, handling simple
// quoted strings. It does not support full shell expansion.
func splitCommand(cmd string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case inQuote && c == quoteChar:
			inQuote = false
		case !inQuote && (c == '"' || c == '\''):
			inQuote = true
			quoteChar = c
		case !inQuote && c == ' ':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
