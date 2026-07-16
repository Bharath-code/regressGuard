// Package explainrun implements the rg explain command.
// It shows the before (snapshot) and after (live) response for a specific
// route with field-level diff highlighting.
package explainrun

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Bharath-code/regressguard/internal/config"
	"github.com/Bharath-code/regressguard/internal/engine"
	"github.com/Bharath-code/regressguard/internal/failures"
	"github.com/Bharath-code/regressguard/internal/snapshot"
	"github.com/Bharath-code/regressguard/internal/ui"
)

// Options configures an explain run.
type Options struct {
	ProjectRoot string
	Route       string // e.g. "GET /api/users"
	JSON        bool
	Stdout      io.Writer
	Stderr      io.Writer
}

// Result is the machine-readable outcome of rg explain.
type Result struct {
	Route   string       `json:"route"`
	Status  string       `json:"status"` // "changed", "unchanged", "error"
	Before  RouteState   `json:"before"`
	After   RouteState   `json:"after"`
	Changes []ChangeItem `json:"changes,omitempty"`
}

// RouteState holds the captured state for one side of the comparison.
type RouteState struct {
	Status     int             `json:"status"`
	SchemaHash string          `json:"schemaHash"`
	Schema     json.RawMessage `json:"schema,omitempty"`
	MS         int64           `json:"ms"`
}

// ChangeItem describes a single difference between before and after.
type ChangeItem struct {
	Type    string `json:"type"`              // "status", "schema", "timing", "field"
	Field   string `json:"field,omitempty"`   // for field-level changes
	Action  string `json:"action,omitempty"`  // "removed", "added", "changed"
	Before  string `json:"before,omitempty"`
	After   string `json:"after,omitempty"`
	Message string `json:"message"`
}

// Run executes the explain pipeline for a single route.
func Run(opts Options) (Result, error) {
	opts = withDefaults(opts)

	// Validate and parse route key.
	routeKey, err := parseRouteKey(opts.Route)
	if err != nil {
		return Result{}, err
	}

	// Load config.
	cfg, err := loadConfig(opts.ProjectRoot)
	if err != nil {
		return Result{}, err
	}

	// Load snapshot.
	snap, err := loadSnapshot(opts.ProjectRoot)
	if err != nil {
		return Result{}, err
	}

	// Find the route in the snapshot.
	snapRecord, ok := snap.Routes[routeKey]
	if !ok {
		return Result{}, failures.Actionable{
			Title:       fmt.Sprintf("rg explain failed: route %q not found in snapshot.", routeKey),
			Cause:       "This route was not captured in the last snapshot.",
			Next:        "rg snapshot",
			MoreContext: "rg explain --help",
		}
	}

	// Find the route in config to get body/skip info.
	var cfgRoute config.Route
	found := false
	for _, r := range cfg.Routes {
		if snapshot.RouteKey(r.Method, r.Path) == routeKey {
			cfgRoute = r
			found = true
			break
		}
	}
	if !found {
		return Result{}, failures.Actionable{
			Title:       fmt.Sprintf("rg explain failed: route %q not in config.", routeKey),
			Cause:       "This route exists in the snapshot but was removed from config.",
			Next:        "rg init --yes",
			MoreContext: "rg explain --help",
		}
	}

	// Check server reachability.
	if !engine.ServerReachable(cfg.ServerURL) {
		return Result{}, failures.Actionable{
			Title:       "rg explain failed: dev server is not responding.",
			Cause:       "The server at " + cfg.ServerURL + " did not respond after several attempts.",
			Next:        "npm run dev",
			MoreContext: "rg doctor",
		}
	}

	// Hit the route live.
	hitOpts := engine.HitOptions{
		ServerURL:    cfg.ServerURL,
		Auth:         cfg.Auth,
		IgnoreFields: cfg.IgnoreFields,
		Verbose:      true, // always capture body for explain
	}
	routeResults := engine.HitRoutes([]config.Route{cfgRoute}, hitOpts, nil)
	if len(routeResults) == 0 || routeResults[0].Skipped {
		reason := "unknown"
		if len(routeResults) > 0 {
			reason = routeResults[0].SkipReason
		}
		return Result{}, failures.Actionable{
			Title:       fmt.Sprintf("rg explain failed: could not hit route %s.", routeKey),
			Cause:       reason,
			Next:        "rg check --verbose",
			MoreContext: "rg explain --help",
		}
	}

	liveResult := routeResults[0]

	// Build the result.
	before := RouteState{
		Status:     snapRecord.Status,
		SchemaHash: snapRecord.SchemaHash,
		Schema:     snapRecord.NormalizedSchema,
		MS:         snapRecord.MS,
	}
	after := RouteState{
		Status:     liveResult.Status,
		SchemaHash: liveResult.SchemaHash,
		Schema:     liveResult.NormalizedSchema,
		MS:         liveResult.MS,
	}

	// Compute changes.
	changes := computeChanges(snapRecord, liveResult)

	status := "unchanged"
	if len(changes) > 0 {
		status = "changed"
	}

	result := Result{
		Route:   routeKey,
		Status:  status,
		Before:  before,
		After:   after,
		Changes: changes,
	}

	// Render output.
	if opts.JSON {
		return result, writeJSON(opts.Stdout, result)
	}
	return result, writeHuman(opts.Stdout, result, snapRecord, liveResult)
}

// computeChanges compares the snapshot record against the live result.
func computeChanges(snap snapshot.RouteRecord, live engine.RouteResult) []ChangeItem {
	var changes []ChangeItem

	// Status change.
	if snap.Status != live.Status {
		changes = append(changes, ChangeItem{
			Type:    "status",
			Before:  fmt.Sprintf("%d", snap.Status),
			After:   fmt.Sprintf("%d", live.Status),
			Message: fmt.Sprintf("Status code: %d -> %d", snap.Status, live.Status),
		})
	}

	// Schema change with field-level detail.
	if snap.SchemaHash != live.SchemaHash && snap.SchemaHash != "" && live.SchemaHash != "" {
		fieldChanges := engine.DiffSchemaShapes(snap.NormalizedSchema, live.NormalizedSchema)
		if len(fieldChanges) > 0 {
			for _, fc := range fieldChanges {
				changes = append(changes, ChangeItem{
					Type:    "field",
					Field:   fc.Field,
					Action:  fc.Action,
					Before:  fc.Before,
					After:   fc.After,
					Message: formatFieldMessage(fc),
				})
			}
		} else {
			changes = append(changes, ChangeItem{
				Type:    "schema",
				Before:  snap.SchemaHash[:8],
				After:   live.SchemaHash[:8],
				Message: "Schema hash changed (structure differs)",
			})
		}
	}

	// Timing change.
	if snap.MS > 0 {
		delta := live.MS - snap.MS
		if delta > 200 && float64(delta)/float64(snap.MS) > 0.5 {
			changes = append(changes, ChangeItem{
				Type:    "timing",
				Before:  fmt.Sprintf("%dms", snap.MS),
				After:   fmt.Sprintf("%dms", live.MS),
				Message: fmt.Sprintf("Response time: %dms -> %dms (+%dms)", snap.MS, live.MS, delta),
			})
		}
	}

	return changes
}

func formatFieldMessage(fc engine.FieldChange) string {
	switch fc.Action {
	case "removed":
		return fmt.Sprintf("Field %q removed (was %s)", fc.Field, fc.Before)
	case "added":
		return fmt.Sprintf("Field %q added (%s)", fc.Field, fc.After)
	case "changed":
		return fmt.Sprintf("Field %q type changed: %s -> %s", fc.Field, fc.Before, fc.After)
	default:
		return fmt.Sprintf("Field %q: %s", fc.Field, fc.Action)
	}
}

// writeHuman renders the explain output for humans.
func writeHuman(w io.Writer, result Result, snap snapshot.RouteRecord, live engine.RouteResult) error {
	lines := []string{
		ui.Header(w, "explain"),
		ui.Separator(w),
		"",
		paint(w, ui.ColorBold, result.Route),
		"",
	}

	// Status comparison.
	statusLabel := "unchanged"
	statusColor := ui.ColorOK
	if snap.Status != live.Status {
		statusLabel = "changed"
		statusColor = ui.ColorFail
	}
	lines = append(lines, fmt.Sprintf("  %-12s %s  %s  %s",
		"Status",
		paint(w, ui.ColorMuted, fmt.Sprintf("%d", snap.Status)),
		paint(w, ui.ColorMuted, "->"),
		paint(w, statusColor, fmt.Sprintf("%d (%s)", live.Status, statusLabel)),
	))

	// Timing comparison.
	timingLabel := "within tolerance"
	timingColor := ui.ColorOK
	delta := live.MS - snap.MS
	if delta > 200 && snap.MS > 0 && float64(delta)/float64(snap.MS) > 0.5 {
		timingLabel = fmt.Sprintf("+%dms slower", delta)
		timingColor = ui.ColorWarn
	}
	lines = append(lines, fmt.Sprintf("  %-12s %s  %s  %s",
		"Timing",
		paint(w, ui.ColorMuted, fmt.Sprintf("%dms", snap.MS)),
		paint(w, ui.ColorMuted, "->"),
		paint(w, timingColor, fmt.Sprintf("%dms (%s)", live.MS, timingLabel)),
	))

	// Schema comparison.
	schemaLabel := "unchanged"
	schemaColor := ui.ColorOK
	if snap.SchemaHash != live.SchemaHash && snap.SchemaHash != "" && live.SchemaHash != "" {
		schemaLabel = "changed"
		schemaColor = ui.ColorFail
	}
	beforeHash := truncate(snap.SchemaHash, 8)
	afterHash := truncate(live.SchemaHash, 8)
	lines = append(lines, fmt.Sprintf("  %-12s %s  %s  %s",
		"Schema",
		paint(w, ui.ColorMuted, beforeHash),
		paint(w, ui.ColorMuted, "->"),
		paint(w, schemaColor, fmt.Sprintf("%s (%s)", afterHash, schemaLabel)),
	))

	lines = append(lines, "")

	// Field-level diff if schema changed.
	fieldChanges := engine.DiffSchemaShapes(snap.NormalizedSchema, live.NormalizedSchema)
	if len(fieldChanges) > 0 {
		lines = append(lines, paint(w, ui.ColorBold, "  Field changes:"))
		for _, fc := range fieldChanges {
			switch fc.Action {
			case "removed":
				lines = append(lines, paint(w, ui.ColorFail, fmt.Sprintf("    - %s (%s)", fc.Field, fc.Before)))
			case "added":
				lines = append(lines, paint(w, ui.ColorOK, fmt.Sprintf("    + %s (%s)", fc.Field, fc.After)))
			case "changed":
				lines = append(lines, paint(w, ui.ColorWarn, fmt.Sprintf("    ~ %s: %s -> %s", fc.Field, fc.Before, fc.After)))
			}
		}
		lines = append(lines, "")
	}

	// Schema shapes side by side.
	if len(snap.NormalizedSchema) > 0 || len(live.NormalizedSchema) > 0 {
		lines = append(lines, paint(w, ui.ColorBold, "  Snapshot schema:"))
		lines = append(lines, formatSchemaIndented(w, snap.NormalizedSchema, "    ")...)
		lines = append(lines, "")
		lines = append(lines, paint(w, ui.ColorBold, "  Live schema:"))
		lines = append(lines, formatSchemaIndented(w, live.NormalizedSchema, "    ")...)
		lines = append(lines, "")
	}

	// Verdict.
	lines = append(lines, ui.Separator(w))
	if result.Status == "unchanged" {
		lines = append(lines, "")
		lines = append(lines, paint(w, ui.ColorOK, "  No differences detected."))
	} else {
		lines = append(lines, "")
		lines = append(lines, paint(w, ui.ColorWarn, fmt.Sprintf("  %d change(s) detected.", len(result.Changes))))
		lines = append(lines, "")
		lines = append(lines, paint(w, ui.ColorMuted, "  If intentional: rg snapshot"))
	}
	lines = append(lines, "")

	ui.StaggeredPrint(w, lines)
	return nil
}

// formatSchemaIndented pretty-prints a JSON schema with indentation.
func formatSchemaIndented(w io.Writer, schema json.RawMessage, prefix string) []string {
	if len(schema) == 0 {
		return []string{prefix + paint(w, ui.ColorMuted, "(none)")}
	}
	var parsed any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		return []string{prefix + paint(w, ui.ColorMuted, "(unparseable)")}
	}
	pretty, err := json.MarshalIndent(parsed, prefix, "  ")
	if err != nil {
		return []string{prefix + paint(w, ui.ColorMuted, string(schema))}
	}
	var lines []string
	for _, line := range strings.Split(string(pretty), "\n") {
		lines = append(lines, paint(w, ui.ColorMuted, line))
	}
	return lines
}

func writeJSON(w io.Writer, result Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func paint(w io.Writer, color ui.Color, text string) string {
	return ui.Paint(w, color, text)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// parseRouteKey validates and normalizes a route key like "GET /api/users".
func parseRouteKey(route string) (string, error) {
	route = strings.TrimSpace(route)
	if route == "" {
		return "", failures.Actionable{
			Title:       "rg explain failed: no route specified.",
			Cause:       "You must provide a route to explain.",
			Next:        "rg explain \"GET /api/users\"",
			MoreContext: "rg explain --help",
		}
	}

	parts := strings.SplitN(route, " ", 2)
	if len(parts) != 2 {
		return "", failures.Actionable{
			Title:       fmt.Sprintf("rg explain failed: invalid route format %q.", route),
			Cause:       "Route must be in the format \"METHOD /path\" (e.g. \"GET /api/users\").",
			Next:        "rg explain \"GET /api/users\"",
			MoreContext: "rg explain --help",
		}
	}

	method := strings.ToUpper(parts[0])
	path := parts[1]
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return method + " " + path, nil
}

func loadConfig(root string) (config.Config, error) {
	if !config.Exists(root) {
		return config.Config{}, failures.Actionable{
			Title:       "rg explain failed: no config found.",
			Cause:       "RegressGuard has not been initialized for this project.",
			Next:        "rg init",
			MoreContext: "rg explain --help",
		}
	}
	cfg, err := config.Load(root)
	if err != nil {
		return config.Config{}, failures.Actionable{
			Title:       "rg explain failed: config is invalid.",
			Cause:       err.Error(),
			Next:        "rg init --yes",
			MoreContext: "rg explain --help",
		}
	}
	return cfg, nil
}

func loadSnapshot(root string) (snapshot.Snapshot, error) {
	if !snapshot.Exists(root) {
		return snapshot.Snapshot{}, failures.MissingSnapshot()
	}
	snap, err := snapshot.Load(root)
	if err != nil {
		return snapshot.Snapshot{}, failures.Actionable{
			Title:       "rg explain failed: snapshot is unreadable.",
			Cause:       err.Error(),
			Next:        "rg snapshot",
			MoreContext: "rg explain --help",
		}
	}
	return snap, nil
}

func withDefaults(opts Options) Options {
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = "."
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return opts
}
