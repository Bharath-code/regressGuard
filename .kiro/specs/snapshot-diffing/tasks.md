# Implementation Plan

## Overview

This plan implements the `rg diff` command (F4 — Snapshot Diffing) in 15 tasks. Tasks are ordered by dependency: history package first (foundation), then diffrun (command logic), then CLI wiring, and finally integration testing.

## Tasks

- [x] 1. Create history package — data model and index persistence
  - Create `internal/history/history.go` with package declaration and imports
  - Define `IndexEntry` struct with fields: `File string`, `GitCommit string`, `CreatedAt time.Time`, `Tests TestMeta`
  - Define `TestMeta` struct with fields: `Passed int`, `Failed int`
  - Define `Index` struct with field: `Entries []IndexEntry` (ordered newest-first)
  - Define constants: `DirName = "history"`, `IndexFileName = "index.json"`, `DefaultMax = 20`
  - Implement `HistoryDir(projectRoot string) string` — returns `.regressguard/history/` path
  - Implement `IndexPath(projectRoot string) string` — returns `.regressguard/history/index.json` path
  - Implement `LoadIndex(projectRoot string) (Index, error)` — reads and parses index.json, returns empty Index if file doesn't exist
  - Implement `SaveIndex(projectRoot string, idx Index) error` — writes index.json with MarshalIndent
  - Write tests: `TestLoadIndex_nonExistentReturnsEmpty`, `TestSaveAndLoadIndex_roundTrip`, `TestLoadIndex_corruptedReturnsError`
  - Requirements addressed: R1 (AC 2, 6)

- [x] 2. Implement snapshot archival in history package
  - Implement `Archive(projectRoot string, snap snapshot.Snapshot) error` function
  - Generate archive filename as `YYYYMMDD-HHmmss-<gitcommit>.json` using `snap.CreatedAt` and `snap.GitCommit`
  - Create `.regressguard/history/` directory with 0755 permissions if it doesn't exist
  - Write the snapshot JSON to the history directory
  - Load existing index (or create empty), prepend new entry, save index
  - Write tests: `TestArchive_createsFileAndUpdatesIndex`, `TestArchive_filenameFormat`, `TestArchive_createsDirectoryIfMissing`
  - Requirements addressed: R1 (AC 1, 2, 6)

- [x] 3. Implement history pruning
  - Implement `Prune(projectRoot string, idx *Index, maxHistory int)` function
  - While `len(idx.Entries) > maxHistory`, remove the last entry (oldest) and delete its file from disk
  - If file deletion fails (already gone), log warning but continue pruning the index
  - Save the updated index after pruning
  - Write tests: `TestPrune_removesOldestEntries`, `TestPrune_noOpWhenUnderLimit`, `TestPrune_handlesAlreadyDeletedFiles`
  - Requirements addressed: R1 (AC 3, 4, 5)

- [x] 4. Implement snapshot reference resolution
  - Implement `Resolve(projectRoot string, ref string, idx Index) (snapshot.Snapshot, ResolvedMeta, error)` function
  - Define `ResolvedMeta` struct with `GitCommit string`, `CreatedAt time.Time`, `Ref string`
  - Handle `"latest"` ref: load `snapshot.json` from project root, return its metadata
  - Handle `~N` ref (regex `^~\d+$`): parse N, validate N >= 1 and N <= len(idx.Entries), load the file at `idx.Entries[N-1]`
  - Handle git commit ref (regex `^[0-9a-fA-F]{4,40}$`): case-insensitive prefix match against all entries
  - Return `failures.Actionable` for: ref not found, ~N out of range, ambiguous commit (multiple matches — list up to 5)
  - Return `failures.Actionable` if referenced file is missing from disk
  - Write tests: `TestResolve_latest`, `TestResolve_tildeN`, `TestResolve_gitCommit`, `TestResolve_outOfRange`, `TestResolve_ambiguousCommit`, `TestResolve_unknownRef`
  - Requirements addressed: R2 (AC 1-6)

- [x] 5. Implement history listing
  - Implement `List(projectRoot string) ([]ListEntry, error)` function
  - Define `ListEntry` struct with `Ref string`, `GitCommit string`, `CreatedAt time.Time`, `TestPassed int`
  - Include current `snapshot.json` as `"latest"` entry at the top
  - Map index entries to `~1`, `~2`, etc.
  - Return `failures.Actionable` if no snapshot exists at all
  - Write tests: `TestList_includesLatestAndHistory`, `TestList_emptyHistoryOnlyLatest`, `TestList_noSnapshotReturnsError`
  - Requirements addressed: R6 (AC 1-3)

- [x] 6. Hook archival into snapshotrun
  - Import `history` package in `internal/snapshotrun/snapshotrun.go`
  - After `snapshot.Write(opts.ProjectRoot, snap)` succeeds, call `history.Archive(opts.ProjectRoot, snap)`
  - If `Archive` returns error, emit warning to stderr: `"! Snapshot history archive failed: <err>"`
  - After archival, load index and call `history.Prune()` with `getMaxHistory(cfg)` (default 20)
  - Add helper `getMaxHistory(cfg config.Config) int` — returns `cfg.MaxHistory` if 1-100, else 20
  - Write test: `TestSnapshotRun_archivesOnSuccess` using `t.TempDir()` fixture
  - Requirements addressed: R1 (AC 1, 3, 4, 5, 7, 8)

- [ ] 7. Add MaxHistory to config
  - Add `MaxHistory int` field to `config.Config` struct with JSON tag `"maxHistory,omitempty"`
  - Ensure `rg config get maxHistory` and `rg config set maxHistory 10` work via existing configrun
  - Write test: `TestConfig_maxHistoryRoundTrip`
  - Requirements addressed: R1 (AC 5, 8)

- [ ] 8. Create diffrun package — core Run function
  - Create `internal/diffrun/diffrun.go` with package declaration
  - Define `Options` struct: `ProjectRoot`, `Args []string`, `JSON bool`, `Verbose bool`, `List bool`, `Stdout io.Writer`, `Stderr io.Writer`
  - Define `Result` struct: `Status string`, `Summary SummaryJSON`, `Results []engine.CheckResult`, `Before SnapshotMeta`, `After SnapshotMeta`, `Next string`
  - Define `SummaryJSON` struct: `Critical int`, `Warnings int`, `Passed int`
  - Define `SnapshotMeta` struct: `GitCommit string`, `CreatedAt string`
  - Implement `Run(opts Options) (Result, error)` with the main flow: list mode, validate args, load index, resolve refs, call DiffSnapshots, build result, render
  - Implement `withDefaults(opts Options) Options` helper
  - Write tests: `TestRun_noArgs_comparesLatestVsPrevious`, `TestRun_oneArg`, `TestRun_twoArgs`, `TestRun_tooManyArgs`
  - Requirements addressed: R3 (AC 1-8)

- [ ] 9. Implement diffrun human-readable output
  - Implement `writeHuman(stdout, stderr io.Writer, result Result, diff engine.DiffResult, beforeMeta, afterMeta history.ResolvedMeta) error`
  - Render header: `ui.Header(stdout, "diff")` + `ui.Separator(stdout)`
  - Render comparison line: `"<commit> (<age>) vs <commit> (<age>)"`
  - Implement `formatAge(t time.Time) string` — returns "now", "3 minutes ago", "1 day ago", "5 days ago"
  - For critical: render `CriticalBanner`, table with Route/Before/After/Change columns, likely cause, next commands
  - For warning: render `WarningBanner`, warning table, next commands
  - For pass: render `PassBanner` with "No changes between snapshots", summary counts
  - For schema findings with FieldChanges: render `+`/`-` prefixed field lines beneath the finding row
  - Use `ui.StaggeredPrint()` for TTY output, plain `writeLines()` for non-TTY
  - Write tests: `TestWriteHuman_critical`, `TestWriteHuman_pass`, `TestWriteHuman_warning` using `bytes.Buffer`
  - Requirements addressed: R4 (AC 1-8)

- [ ] 10. Implement diffrun JSON output
  - Implement `writeJSON(w io.Writer, result Result) error` — encodes Result as indented JSON
  - Implement `writeListJSON(w io.Writer, entries []history.ListEntry) error`
  - Ensure `status` field is one of: "pass", "warning", "critical"
  - Ensure `before`/`after` metadata objects include `gitCommit` and `createdAt`
  - Write tests: `TestWriteJSON_validOutput`, `TestWriteJSON_includesMetadata`
  - Requirements addressed: R5 (AC 1-4)

- [ ] 11. Implement diffrun list rendering
  - Implement `writeHumanList(stdout io.Writer, entries []history.ListEntry) error`
  - Render table with columns: Ref, Commit, Age, Tests
  - Include "latest" at top, then `~1`, `~2`, etc.
  - Show usage hint at bottom: `"rg diff ~2 latest"`
  - Handle empty history: show actionable message with `rg snapshot` as next command
  - Write tests: `TestWriteHumanList_withEntries`, `TestWriteHumanList_empty`
  - Requirements addressed: R6 (AC 1-5)

- [ ] 12. Implement diffrun verbose output
  - Implement `writeVerbose(stderr io.Writer, diff engine.DiffResult, before, after snapshot.Snapshot) error`
  - For schema findings: print full normalized schema (before and after) to stderr
  - For all routes: print timing table (route, before ms, after ms, delta) to stderr
  - For added/removed routes: print route key with `+` or `-` prefix to stderr
  - Ensure verbose output goes to stderr only (never stdout)
  - Write tests: `TestWriteVerbose_schemaDetail`, `TestWriteVerbose_timingTable`
  - Requirements addressed: R7 (AC 1-4)

- [ ] 13. Wire diff command into CLI
  - Add `newDiffCommand()` function in `internal/cli/cli.go`
  - Register command with `root.AddCommand(newDiffCommand())` in `NewRootCommand`
  - Add import for `diffrun` package
  - Set `Use: "diff [before-ref] [after-ref]"`, `Short: "Compare two snapshots"`, `Args: cobra.MaximumNArgs(2)`
  - Wire flags: `--json`, `--verbose`, `--list`
  - Handle exit code 1 on critical result (same pattern as checkrun)
  - Handle JSON error rendering via `writeActionable`
  - Set help template with example: `rg diff ~2 latest`
  - Verify `rg --help` shows `diff` in the command list
  - Requirements addressed: R3 (AC 5-8)

- [ ] 14. Error handling and actionable errors
  - Add `MissingHistory()` function to `internal/failures/actionable.go` returning Actionable with title "No snapshot history available"
  - Add `SnapshotNotFound(ref string)` function returning Actionable with title "Snapshot not found"
  - Add `AmbiguousCommit(matches []string)` function returning Actionable with title "Ambiguous snapshot reference"
  - Add `SnapshotIndexCorrupted(parseErr error)` function returning Actionable with title "Snapshot index is unreadable"
  - Add `SnapshotFileMissing(path string)` function returning Actionable with title "Snapshot file missing"
  - Ensure all errors include `MoreContext: "rg diff --help"` or `"rg doctor"`
  - Write tests: verify each function returns correct Title, Cause, Next, MoreContext
  - Requirements addressed: R8 (AC 1-5)

- [ ] 15. Integration test — full diff workflow
  - Create `internal/diffrun/diffrun_test.go` integration test using `t.TempDir()`
  - Test full workflow: create config, run snapshot (mock), archive, modify snapshot, run snapshot again, run diff
  - Verify diff detects status code change between two archived snapshots
  - Verify `--list` shows both snapshots with correct refs
  - Verify `--json` output is valid JSON with correct schema
  - Verify exit code 0 for pass, exit code 1 for critical (via Result.Status)
  - Verify error case: no history returns actionable error
  - Requirements addressed: R1-R8

## Task Dependency Graph

```json
{
  "waves": [
    {"tasks": [1, 14]},
    {"tasks": [2, 3, 4, 5, 7]},
    {"tasks": [6, 8]},
    {"tasks": [9, 10, 11, 12]},
    {"tasks": [13]},
    {"tasks": [15]}
  ]
}
```

## Notes

- All tests use `t.TempDir()` for isolation — no shared fixtures
- Tasks 2-5 can be developed in parallel once Task 1 is complete
- Tasks 9-12 can be developed in parallel once Task 8 is complete
- Task 14 (error helpers) should be done early as it's used by Tasks 4, 5, and 8
- The `rg diff` command follows the same patterns as `rg check` for output rendering and exit codes
