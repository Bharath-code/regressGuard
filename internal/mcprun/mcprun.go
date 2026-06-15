// Package mcprun implements the rg mcp serve command.
// It exposes snapshot, check, and status as MCP tools over stdio transport,
// allowing AI agents (Claude Code, Cursor, etc.) to call them directly.
package mcprun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Bharath-code/regressguard/internal/checkrun"
	"github.com/Bharath-code/regressguard/internal/snapshotrun"
	"github.com/Bharath-code/regressguard/internal/statusrun"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Options configures the MCP server.
type Options struct {
	Version     string
	ProjectRoot string
}

// auditLogger writes MCP tool invocations to .regressguard/mcp-audit.log.
type auditLogger struct {
	mu       sync.Mutex
	filePath string
}

func newAuditLogger(projectRoot string) *auditLogger {
	return &auditLogger{
		filePath: filepath.Join(projectRoot, ".regressguard", "mcp-audit.log"),
	}
}

func (a *auditLogger) log(toolName string, args map[string]any, status string, durationMs int64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Ensure directory exists.
	dir := filepath.Dir(a.filePath)
	_ = os.MkdirAll(dir, 0o755)

	f, err := os.OpenFile(a.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	entry := map[string]any{
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"tool":       toolName,
		"status":     status,
		"durationMs": durationMs,
	}
	if len(args) > 0 {
		entry["args"] = args
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = f.Write(append(data, '\n'))
}

// Serve starts the MCP server on stdio transport.
// It blocks until the connection is closed.
// S4: validates and restricts operations to the specified project root.
func Serve(opts Options) error {
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = "."
	}

	// S4: resolve to absolute path and validate it exists.
	absRoot, err := filepath.Abs(opts.ProjectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("project root %q is not a valid directory", absRoot)
	}
	opts.ProjectRoot = absRoot

	// S6: create audit logger.
	audit := newAuditLogger(absRoot)

	s := server.NewMCPServer(
		"regressguard",
		opts.Version,
		server.WithToolCapabilities(true),
	)

	// Register tools.
	s.AddTool(checkTool(), makeCheckHandler(opts.ProjectRoot, audit))
	s.AddTool(snapshotTool(), makeSnapshotHandler(opts.ProjectRoot, audit))
	s.AddTool(statusTool(), makeStatusHandler(opts.ProjectRoot, audit))

	// Start stdio transport (blocks until stdin closes).
	return server.ServeStdio(s)
}

// validatePath ensures a path doesn't escape the project root (S4).
func validatePath(projectRoot, requestedPath string) error {
	if requestedPath == "" {
		return nil
	}
	abs, err := filepath.Abs(requestedPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	// Ensure the resolved path is within or equal to the project root.
	// Use filepath.Rel for a true boundary check — a plain prefix compare would
	// wrongly accept a sibling like /root-evil for project root /root.
	rel, err := filepath.Rel(projectRoot, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q is outside the allowed project root %q", requestedPath, projectRoot)
	}
	return nil
}

// --- Tool definitions ---

func checkTool() mcp.Tool {
	return mcp.NewTool("check",
		mcp.WithDescription("Compare current state against the snapshot. Detects regressions in tests, API status codes, response schemas, and timing. Returns structured results with severity levels."),
		mcp.WithString("since",
			mcp.Description("Git ref to scope check to changed routes only (e.g. HEAD~1, main). Optional."),
		),
	)
}

func snapshotTool() mcp.Tool {
	return mcp.NewTool("snapshot",
		mcp.WithDescription("Record the current passing state — tests, routes, response schemas. Creates a baseline for future regression checks. Requires the dev server to be running."),
	)
}

func statusTool() mcp.Tool {
	return mcp.NewTool("status",
		mcp.WithDescription("Quick health check showing snapshot age, route count, config health, and hook status. Sub-second, does not run tests or hit routes."),
	)
}

// --- Tool handlers ---

func makeCheckHandler(projectRoot string, audit *auditLogger) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		since, _ := request.GetArguments()["since"].(string)

		result, err := checkrun.Run(checkrun.Options{
			ProjectRoot: projectRoot,
			JSON:        true,
			Since:       since,
			Stdout:      io.Discard,
			Stderr:      io.Discard,
		})

		status := "success"
		if err != nil {
			status = "error"
			audit.log("check", request.GetArguments(), status, time.Since(start).Milliseconds())
			return mcp.NewToolResultError(err.Error()), nil
		}

		audit.log("check", request.GetArguments(), status, time.Since(start).Milliseconds())
		return toolResultFromStruct(result)
	}
}

func makeSnapshotHandler(projectRoot string, audit *auditLogger) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()

		result, err := snapshotrun.Run(snapshotrun.Options{
			ProjectRoot: projectRoot,
			JSON:        true,
			Stdout:      io.Discard,
			Stderr:      io.Discard,
		})

		status := "success"
		if err != nil {
			status = "error"
			audit.log("snapshot", request.GetArguments(), status, time.Since(start).Milliseconds())
			return mcp.NewToolResultError(err.Error()), nil
		}

		audit.log("snapshot", request.GetArguments(), status, time.Since(start).Milliseconds())
		return toolResultFromStruct(result)
	}
}

func makeStatusHandler(projectRoot string, audit *auditLogger) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()

		result, err := statusrun.Run(statusrun.Options{
			ProjectRoot: projectRoot,
			JSON:        true,
			Stdout:      io.Discard,
			Stderr:      io.Discard,
		})

		status := "success"
		if err != nil {
			status = "error"
			audit.log("status", request.GetArguments(), status, time.Since(start).Milliseconds())
			return mcp.NewToolResultError(err.Error()), nil
		}

		audit.log("status", request.GetArguments(), status, time.Since(start).Milliseconds())
		return toolResultFromStruct(result)
	}
}

// toolResultFromStruct serializes any struct as JSON text content.
func toolResultFromStruct(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(data)), nil
}
