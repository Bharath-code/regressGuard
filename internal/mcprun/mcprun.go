// Package mcprun implements the rg mcp serve command.
// It exposes snapshot, check, and status as MCP tools over stdio transport,
// allowing AI agents (Claude Code, Cursor, etc.) to call them directly.
package mcprun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

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

// Serve starts the MCP server on stdio transport.
// It blocks until the connection is closed.
func Serve(opts Options) error {
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = "."
	}

	s := server.NewMCPServer(
		"regressguard",
		opts.Version,
		server.WithToolCapabilities(true),
	)

	// Register tools.
	s.AddTool(checkTool(), makeCheckHandler(opts.ProjectRoot))
	s.AddTool(snapshotTool(), makeSnapshotHandler(opts.ProjectRoot))
	s.AddTool(statusTool(), makeStatusHandler(opts.ProjectRoot))

	// Start stdio transport (blocks until stdin closes).
	return server.ServeStdio(s)
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

func makeCheckHandler(projectRoot string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		since, _ := request.GetArguments()["since"].(string)

		result, err := checkrun.Run(checkrun.Options{
			ProjectRoot: projectRoot,
			JSON:        true,
			Since:       since,
			Stdout:      io.Discard,
			Stderr:      io.Discard,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return toolResultFromStruct(result)
	}
}

func makeSnapshotHandler(projectRoot string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := snapshotrun.Run(snapshotrun.Options{
			ProjectRoot: projectRoot,
			JSON:        true,
			Stdout:      io.Discard,
			Stderr:      io.Discard,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return toolResultFromStruct(result)
	}
}

func makeStatusHandler(projectRoot string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := statusrun.Run(statusrun.Options{
			ProjectRoot: projectRoot,
			JSON:        true,
			Stdout:      io.Discard,
			Stderr:      io.Discard,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

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
