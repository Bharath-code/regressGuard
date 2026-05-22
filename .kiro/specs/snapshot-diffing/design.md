# Design Document

## Overview

This design implements the `rg diff` command (F4 — Snapshot Diffing) which enables developers to compare two stored snapshots against each other. The feature requires two new packages (`internal/diffrun` for the command orchestration and `internal/history` for snapshot history management), modifications to the existing `snapshotrun` package to archive snapshots on save, and a new Cobra command wired into `cli.go`.

The design reuses the existing `engine.DiffSnapshots` function for comparison logic and follows the established patterns from `checkrun` for output rendering.

## Architecture

### System Context

```
┌─────────────────────────────────────────────────────────────────┐
│                        rg diff command                           │
├─────────────┬──────────────────┬────────────────────────────────┤
│  CLI Layer  │  Diff Runner     │  History Manager                │
│  (cli.go)   │  (diffrun/)      │  (history/)                    │
│             │                  │                                 │
│  Cobra cmd  │  Resolve refs    │  Archive snapshots              │
│  Parse args │  Load snapshots  │  Maintain index.json            │
│  Wire flags │  Call DiffEngine │  Prune old entries              │
│             │  Render output   │  List available snapshots       │
│             │                  │                                 │
└─────────────┴──────────────────┴────────────────────────────────┘
                      │                       │
                      ▼                       ▼
              ┌──────────────┐      ┌──────────────────────┐
              │ engine.Diff  │      │ .regressguard/       │
              │ Snapshots()  │      │   snapshot.json      │
              │              │      │   history/            │
              └──────────────┘      │     index.json       │
                                    │     20260520-*.json   │
                                    └──────────────────────┘
```

### Package Structure

```
internal/
  history/           # NEW — snapshot history persistence
    history.go       # Archive, Load, List, Prune, Resolve
    history_test.go
  diffrun/           # NEW — rg diff command orchestration
    diffrun.go       # Run(), Options, Result, rendering
    diffrun_test.go
```

## Components and Interfaces

### Component 1: History Manager (`internal/history`)

**Purpose:** Manages the `.regressguard/history/` directory, the `index.json` file, and provides snapshot reference resolution.

#### Data Model — Index Entry

```go
// IndexEntry represents one archived snapshot in the history index.
type IndexEntry struct {
    File      string    `json:"file"`      // relative path: "20260520-133138-2b25619.json"
    GitCommit string    `json:"gitCommit"` // short hash or "unknown"
    CreatedAt time.Time `json:"createdAt"` // RFC 3339
    Tests     TestMeta  `json:"tests"`     // summary for listing
}

type TestMeta struct {
    Passed int `json:"passed"`
    Failed int `json:"failed"`
}

// Index is the full history index file.
type Index struct {
    Entries []IndexEntry `json:"entries"` // ordered newest-first
}
```

#### Interface

```go
package history

const (
    DirName       = "history"
    IndexFileName = "index.json"
    DefaultMax    = 20
)

// Archive copies the current snapshot to history and updates the index.
func Archive(projectRoot string, snap snapshot.Snapshot) error

// LoadIndex reads and parses .regressguard/history/index.json.
func LoadIndex(projectRoot string) (Index, error)

// SaveIndex writes the index back to disk.
func SaveIndex(projectRoot string, idx Index) error

// Prune removes entries beyond maxHistory, deleting files and updating index.
func Prune(projectRoot string, idx *Index, maxHistory int)

// Resolve resolves a Snapshot_Ref string to a loaded Snapshot.
func Resolve(projectRoot string, ref string, idx Index) (snapshot.Snapshot, ResolvedMeta, error)

// ResolvedMeta contains metadata about the resolved snapshot for display.
type ResolvedMeta struct {
    GitCommit string
    CreatedAt time.Time
    Ref       string
}

// List returns all available snapshots (latest + history) for display.
func List(projectRoot string) ([]ListEntry, error)

type ListEntry struct {
    Ref        string
    GitCommit  string
    CreatedAt  time.Time
    TestPassed int
}
```

#### Archive File Naming

Format: `YYYYMMDD-HHmmss-<gitcommit>.json`

Example: `20260520-133138-2b25619.json`

This ensures lexicographic sorting matches chronological order and avoids collisions.

#### Reference Resolution Logic

1. `"latest"` → load `snapshot.json` directly
2. `"~N"` (regex: `^~\d+$`) → index into `Index.Entries[N-1]` (0-indexed internally, 1-indexed for user)
3. Hex string (regex: `^[0-9a-fA-F]{4,40}$`) → case-insensitive prefix match on `IndexEntry.GitCommit`
4. Anything else → return actionable error "Snapshot not found"

### Component 2: Diff Runner (`internal/diffrun`)

**Purpose:** Orchestrates the `rg diff` command — resolves references, loads snapshots, runs the diff engine, and renders output.

#### Interface

```go
package diffrun

type Options struct {
    ProjectRoot string
    Args        []string // positional args (0, 1, or 2 refs)
    JSON        bool
    Verbose     bool
    List        bool
    Stdout      io.Writer
    Stderr      io.Writer
}

type Result struct {
    Status  string               `json:"status"`
    Summary SummaryJSON          `json:"summary"`
    Results []engine.CheckResult `json:"results"`
    Before  SnapshotMeta         `json:"before"`
    After   SnapshotMeta         `json:"after"`
    Next    string               `json:"next"`
}

type SummaryJSON struct {
    Critical int `json:"critical"`
    Warnings int `json:"warnings"`
    Passed   int `json:"passed"`
}

type SnapshotMeta struct {
    GitCommit string `json:"gitCommit"`
    CreatedAt string `json:"createdAt"`
}

// Run executes the diff pipeline and returns a Result.
func Run(opts Options) (Result, error)
```

#### Run Function Flow

```
Run(opts) → Result, error
  1. If opts.List → call history.List(), render list, return
  2. Validate args (max 2 positional)
  3. Load history index
  4. Resolve "before" ref (default: ~1 if no args)
  5. Resolve "after" ref (default: latest)
  6. Call engine.DiffSnapshots(before, after)
  7. Build Result from DiffResult
  8. Render (JSON or human)
  9. Return Result (caller handles exit code)
```

### Component 3: CLI Wiring (`internal/cli/cli.go`)

Add `newDiffCommand()` to the root command list:

```go
func newDiffCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "diff [before-ref] [after-ref]",
        Short: "Compare two snapshots",
        Args:  cobra.MaximumNArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            jsonMode, _ := cmd.Flags().GetBool("json")
            verbose, _ := cmd.Flags().GetBool("verbose")
            list, _ := cmd.Flags().GetBool("list")

            result, err := diffrun.Run(diffrun.Options{
                ProjectRoot: ".",
                Args:        args,
                JSON:        jsonMode,
                Verbose:     verbose,
                List:        list,
                Stdout:      cmd.OutOrStdout(),
                Stderr:      cmd.ErrOrStderr(),
            })
            if err != nil {
                if issue, ok := err.(failures.Actionable); ok {
                    if jsonMode {
                        return writeActionable(cmd, issue)
                    }
                    return issue
                }
                return err
            }
            if result.Status == "critical" {
                os.Exit(1)
            }
            return nil
        },
    }
    cmd.SetHelpTemplate(commandHelpTemplate("rg diff ~2 latest"))
    cmd.Flags().Bool("json", false, "write machine-readable JSON to stdout")
    cmd.Flags().Bool("verbose", false, "write detailed diagnostics to stderr")
    cmd.Flags().Bool("list", false, "list available snapshots")
    return cmd
}
```

### Component 4: Snapshot Archival Hook (`internal/snapshotrun`)

Modify `snapshotrun.Run()` to call `history.Archive()` after a successful snapshot write:

```go
// After snapshot.Write(opts.ProjectRoot, snap) succeeds:
if err := history.Archive(opts.ProjectRoot, snap); err != nil {
    fmt.Fprintf(opts.Stderr, "%s Snapshot history archive failed: %v\n",
        ui.Paint(opts.Stderr, ui.ColorWarn, ui.SymbolWarning), err)
}
if idx, loadErr := history.LoadIndex(opts.ProjectRoot); loadErr == nil {
    maxHist := getMaxHistory(cfg)
    history.Prune(opts.ProjectRoot, &idx, maxHist)
    _ = history.SaveIndex(opts.ProjectRoot, idx)
}
```

### Component 5: Config Extension

Add `MaxHistory` field to `config.Config`:

```go
type Config struct {
    // ... existing fields ...
    MaxHistory int `json:"maxHistory,omitempty"` // 0 means use default (20)
}
```

## Data Models

### History Index File (`.regressguard/history/index.json`)

```json
{
  "entries": [
    {
      "file": "20260520-133138-2b25619.json",
      "gitCommit": "2b25619",
      "createdAt": "2026-05-20T13:31:38Z",
      "tests": { "passed": 4, "failed": 0 }
    },
    {
      "file": "20260519-091022-a1f8c02.json",
      "gitCommit": "a1f8c02",
      "createdAt": "2026-05-19T09:10:22Z",
      "tests": { "passed": 4, "failed": 0 }
    }
  ]
}
```

### JSON Output Schema (`rg diff --json`)

```json
{
  "status": "critical",
  "summary": {
    "critical": 2,
    "warnings": 0,
    "passed": 4
  },
  "results": [
    {
      "severity": "CRITICAL",
      "type": "status",
      "route": "GET /api/auth/verify",
      "before": 200,
      "after": 401,
      "message": "GET /api/auth/verify: status 200 -> 401"
    }
  ],
  "before": {
    "gitCommit": "a1f8c02",
    "createdAt": "2026-05-19T09:10:22Z"
  },
  "after": {
    "gitCommit": "2b25619",
    "createdAt": "2026-05-20T13:31:38Z"
  },
  "next": "rg diff --verbose"
}
```

### List Output Schema (`rg diff --list --json`)

```json
[
  { "ref": "latest", "gitCommit": "2b25619", "createdAt": "2026-05-20T13:31:38Z", "testsPassed": 4 },
  { "ref": "~1", "gitCommit": "a1f8c02", "createdAt": "2026-05-19T09:10:22Z", "testsPassed": 4 },
  { "ref": "~2", "gitCommit": "f3e9b11", "createdAt": "2026-05-18T15:42:01Z", "testsPassed": 3 }
]
```

## Human Output Screens

### Diff — Critical Findings

```
RegressGuard  diff
──────────────────────────────────────────────────────

a1f8c02 (1 day ago) vs 2b25619 (now)

X CRITICAL 2 regressions between snapshots

Route                          Before  After   Change
GET /api/auth/verify           200     401     status
POST /api/user/update          200     500     status

Likely cause:
  Auth/session behavior changed between these snapshots.

Next:
  rg diff --verbose
  rg explain "GET /api/auth/verify"
```

### Diff — No Changes

```
RegressGuard  diff
──────────────────────────────────────────────────────

a1f8c02 (1 day ago) vs 2b25619 (now)

OK PASS No changes between snapshots

Tests       4 passed, 0 failed (both)
Routes      5 unchanged
```

### Diff — List

```
RegressGuard  diff --list
──────────────────────────────────────────────────────

Ref      Commit    Age            Tests
latest   2b25619   now            4 passed
~1       a1f8c02   1 day ago      4 passed
~2       f3e9b11   3 days ago     3 passed

Usage:
  rg diff ~2 latest
```

## Correctness Properties

### Property 1: Deterministic output
Given the same two snapshots, `rg diff` always produces the same DiffResult regardless of system time or environment.

**Validates: Requirement 3.4**

### Property 2: Index consistency
The index.json always reflects the actual files on disk. If a file is missing, it is detected at resolve time and reported as an actionable error.

**Validates: Requirement 1.2**

### Property 3: Non-destructive archival
Archival failure never prevents the primary `rg snapshot` operation from succeeding.

**Validates: Requirement 1.7**

### Property 4: Ordering invariant
`Index.Entries` is always ordered newest-first. Pruning always removes from the tail (oldest).

**Validates: Requirement 1.3**

### Property 5: Ref uniqueness
`~N` always resolves to exactly one snapshot. Git commit refs must resolve to exactly one match or produce an ambiguity error.

**Validates: Requirement 2.1**

## Testing Strategy

- **Unit tests** for `history` package: index CRUD, archival, pruning, reference resolution (all using `t.TempDir()`)
- **Unit tests** for `diffrun` package: Run function with mock snapshots, output rendering to `bytes.Buffer`
- **Integration test**: full workflow (config → snapshot → archive → snapshot → archive → diff) in a temp directory
- **No TTY dependency**: all tests use `bytes.Buffer` for stdout/stderr
- **Slow test gating**: integration tests gated with `if testing.Short() { t.Skip() }`

## Error Handling

All errors use `failures.Actionable{}`:

| Condition | Title | Next |
|-----------|-------|------|
| No history exists | "No snapshot history available" | `rg snapshot` |
| Ref out of range | "Snapshot not found" | `rg diff --list` |
| Ambiguous commit | "Ambiguous snapshot reference" | provide longer prefix |
| Index corrupted | "Snapshot index is unreadable" | `rg snapshot` |
| File missing | "Snapshot file missing" | `rg snapshot` |
| Too many args | "Too many arguments" | `rg diff --help` |

## Key Design Decisions

1. **History is automatic** — `rg snapshot` always archives. No opt-in required. This ensures `rg diff` works out of the box after the second snapshot.

2. **Index file for fast lookups** — Rather than scanning the history directory, we maintain `index.json` for O(1) ref resolution and ordered listing.

3. **Non-blocking archival** — If history archival fails (disk full, permissions), the snapshot itself still succeeds. History is a convenience feature, not a gate.

4. **Reuse DiffSnapshots** — The existing diff engine handles all comparison logic. `diffrun` only handles I/O, ref resolution, and rendering.

5. **~N syntax** — Inspired by git's `HEAD~N` syntax. Familiar to developers, concise, and unambiguous.

6. **Default retention of 20** — Covers ~3 weeks of daily snapshots. Configurable via `maxHistory` for power users.

## Traceability

| Requirement | Design Component |
|-------------|-----------------|
| R1: Snapshot History Storage | `history.Archive()`, `history.Prune()`, index.json data model |
| R2: Snapshot Reference Resolution | `history.Resolve()`, ref parsing logic |
| R3: Diff Command Execution | `diffrun.Run()`, CLI wiring in `newDiffCommand()` |
| R4: Human-Readable Diff Output | `diffrun.writeHuman*()` functions |
| R5: Machine-Readable Diff Output | `diffrun.writeJSON()`, Result struct |
| R6: Snapshot Listing | `history.List()`, `diffrun` list rendering |
| R7: Verbose Diff Output | `diffrun.writeVerbose()` to stderr |
| R8: Error Handling | `failures.Actionable` returns throughout |
