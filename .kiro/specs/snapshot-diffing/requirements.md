# Requirements Document

## Introduction

The `rg diff` command enables developers to compare two stored snapshots against each other (snapshot vs snapshot), rather than comparing a snapshot against the current live state. This is essential for understanding what changed across AI coding sessions — answering questions like "what did the AI break between Tuesday and today?" without needing to re-run tests or hit routes.

Currently, RegressGuard stores a single snapshot at `.regressguard/snapshot.json`. This feature introduces snapshot history so that `rg snapshot` automatically retains previous snapshots, and `rg diff` can reference them by index, git commit, or label.

## Glossary

- **Snapshot_Store**: The persistence layer that manages multiple snapshots in `.regressguard/history/` as individual timestamped JSON files
- **Diff_Command**: The `rg diff` CLI command that loads two snapshots and produces a structured comparison
- **Diff_Renderer**: The component that formats DiffResult into human-readable or JSON output for the diff command
- **Snapshot_Ref**: A user-provided reference to identify a snapshot — can be a numeric index (e.g., `~1` for previous), a git commit short hash, or the keyword `latest`
- **Snapshot_Index**: The ordered list of available snapshots maintained in `.regressguard/history/index.json`
- **History_Pruner**: The component that enforces the maximum snapshot retention limit

## Requirements

### Requirement 1: Snapshot History Storage

**User Story:** As a developer, I want my previous snapshots to be retained automatically, so that I can compare them later without manual bookkeeping.

#### Acceptance Criteria

1. WHEN `rg snapshot` completes successfully, THE Snapshot_Store SHALL copy the current snapshot to `.regressguard/history/<timestamp>-<gitcommit>.json` before overwriting `snapshot.json`, where `<timestamp>` is formatted as `YYYYMMDD-HHmmss` in UTC and `<gitcommit>` is the short hash returned by `git rev-parse --short HEAD` (or the literal string `unknown` if unavailable)
2. WHEN a snapshot is archived, THE Snapshot_Store SHALL append an entry to the Snapshot_Index file located at `.regressguard/history/index.json`, containing the relative file path, git commit string, and creation timestamp in RFC 3339 format
3. WHILE the number of files in `.regressguard/history/` (excluding `index.json`) exceeds the retention limit, THE History_Pruner SHALL delete the file with the earliest creation timestamp according to the Snapshot_Index and remove its corresponding entry from the Snapshot_Index
4. THE Snapshot_Store SHALL enforce a default retention limit of 20 historical snapshots
5. WHERE the `maxHistory` config option is set to an integer between 1 and 100 inclusive, THE History_Pruner SHALL use that value as the retention limit instead of the default 20
6. IF the `.regressguard/history/` directory does not exist, THEN THE Snapshot_Store SHALL create it with permissions 0755
7. IF writing the archived snapshot file or updating the Snapshot_Index fails, THEN THE Snapshot_Store SHALL proceed with overwriting `snapshot.json` and emit a warning to stderr indicating the archive operation failed
8. IF the `maxHistory` config value is present but is not an integer between 1 and 100, THEN THE Snapshot_Store SHALL ignore the invalid value, use the default of 20, and emit a warning to stderr

### Requirement 2: Snapshot Reference Resolution

**User Story:** As a developer, I want to reference snapshots by index, git commit, or keyword, so that I can quickly specify which snapshots to compare.

#### Acceptance Criteria

1. WHEN a Snapshot_Ref of `~N` is provided (where N is a positive integer), THE Diff_Command SHALL resolve it to the Nth most recent snapshot in the Snapshot_Index, where `~1` is the most recent historical snapshot and `~2` is the second most recent
2. WHEN a Snapshot_Ref matching a git commit short hash (4-40 hex characters) is provided, THE Diff_Command SHALL perform a case-insensitive prefix match against the git commit field of all entries in the Snapshot_Index and resolve to the matching snapshot
3. WHEN the keyword `latest` is provided as a Snapshot_Ref, THE Diff_Command SHALL resolve it to the current `snapshot.json`
4. IF a Snapshot_Ref of `~N` is provided where N exceeds the number of available historical snapshots, THEN THE Diff_Command SHALL return an actionable error with the title "Snapshot not found", the cause "Only M historical snapshots available, but ~N was requested", and the next command `rg diff --list`
5. IF a Snapshot_Ref does not match any stored snapshot (neither index, commit hash, nor keyword), THEN THE Diff_Command SHALL return an actionable error with the title "Snapshot not found", the cause describing the unresolved reference, and the next command `rg diff --list`
6. IF multiple snapshots match an ambiguous git commit prefix, THEN THE Diff_Command SHALL return an actionable error listing up to 5 matching commits and suggesting a longer prefix

### Requirement 3: Diff Command Execution

**User Story:** As a developer, I want to run `rg diff` to see what changed between two snapshots, so that I can understand the impact of recent AI coding sessions.

#### Acceptance Criteria

1. WHEN `rg diff` is invoked with no arguments, THE Diff_Command SHALL compare the most recent historical snapshot (`~1`) as the before snapshot against the current snapshot (`latest`) as the after snapshot
2. WHEN `rg diff <ref>` is invoked with one argument, THE Diff_Command SHALL compare the resolved snapshot as the before snapshot against the current snapshot (`latest`) as the after snapshot
3. WHEN `rg diff <ref1> <ref2>` is invoked with two arguments, THE Diff_Command SHALL compare the first resolved snapshot (before) against the second resolved snapshot (after)
4. IF `rg diff` is invoked with more than two positional arguments, THEN THE Diff_Command SHALL exit with code 2 and return an actionable error indicating that at most two snapshot references are accepted
5. THE Diff_Command SHALL reuse the existing `engine.DiffSnapshots` function to compute the DiffResult
6. IF the DiffResult contains one or more critical findings, THEN THE Diff_Command SHALL exit with code 1
7. IF the DiffResult contains no critical findings (including zero findings or warnings only), THEN THE Diff_Command SHALL exit with code 0
8. IF a usage error or snapshot resolution error occurs, THEN THE Diff_Command SHALL exit with code 2

### Requirement 4: Human-Readable Diff Output

**User Story:** As a developer, I want a clear terminal display of what changed between snapshots, so that I can quickly assess the impact.

#### Acceptance Criteria

1. THE Diff_Renderer SHALL display a header identifying the baseline snapshot by its short git commit hash (7 characters) and age relative to the current time (e.g., "2b25619 (3 days ago) vs a1f8c02 (now)"), and IF the baseline git commit is unavailable, THEN THE Diff_Renderer SHALL display "unknown" in place of the commit hash
2. WHEN critical findings exist, THE Diff_Renderer SHALL display each finding in a table with four columns: Route (max 36 characters, truncated with `~`), Before value, After value, and Change type (one of: status, schema, tests)
3. WHEN schema changes exist with field-level detail, THE Diff_Renderer SHALL display added fields prefixed with `+` and removed fields prefixed with `-`, each on its own line beneath the parent finding row
4. WHEN no differences are found between the two snapshots, THE Diff_Renderer SHALL display "No changes between snapshots" with a PASS banner
5. THE Diff_Renderer SHALL display a summary line with integer counts of critical findings, warnings, and passed routes (routes with no critical finding)
6. WHILE the output stream is a TTY with color enabled, THE Diff_Renderer SHALL use `ui.Paint()` for colors, `ui.StaggeredPrint()` for line-by-line reveals, and constrain all output lines to a maximum of 80 characters
7. WHILE the output stream is non-TTY or NO_COLOR is set, THE Diff_Renderer SHALL emit plain text with no ANSI escape codes and no animated delays
8. WHEN only warning-level findings exist (no critical findings), THE Diff_Renderer SHALL display each warning with route and change description, and render a WARNING banner instead of a PASS or CRITICAL banner

### Requirement 5: Machine-Readable Diff Output

**User Story:** As a developer, I want JSON output from `rg diff --json`, so that I can integrate snapshot comparison into CI scripts and automation.

#### Acceptance Criteria

1. WHEN `--json` is passed, THE Diff_Command SHALL write a single JSON object to stdout containing the fields: `status` (one of "pass", "warning", "critical", "error"), `summary` (with integer counts for `critical`, `warnings`, and `passed`), `results` (an array of finding objects, each with `severity`, `type`, `route`, `before`, `after`, and `message`), `before` and `after` metadata objects, and `next` (a suggested follow-up command string)
2. WHEN `--json` is passed, THE Diff_Command SHALL write progress indicators, diagnostic messages, and warnings only to stderr so that stdout remains valid JSON parseable by tools such as jq
3. WHEN `--json` is passed, THE Diff_Command SHALL include `before` and `after` metadata objects in the JSON output, where each object contains `gitCommit` (the short SHA of the snapshot's recorded commit, or "unknown" if unavailable) and `createdAt` (the ISO 8601 UTC timestamp of when that snapshot was captured)
4. IF the diff pipeline encounters a configuration, snapshot, or runtime error while `--json` is passed, THEN THE Diff_Command SHALL write a JSON object to stdout with `status` set to "error" and an `error` object containing `title` and `next` fields, and SHALL exit with code 2

### Requirement 6: Snapshot Listing

**User Story:** As a developer, I want to see all available snapshots, so that I can choose which ones to compare.

#### Acceptance Criteria

1. WHEN `rg diff --list` is invoked, THE Diff_Command SHALL display all snapshots in the Snapshot_Index ordered from newest to oldest to stdout, including the current `snapshot.json` labeled as `latest` at the top of the list
2. THE Diff_Command SHALL display each snapshot entry with its index reference (`~1`, `~2`, etc., with `latest` for the current snapshot), git commit short hash (7 characters), creation timestamp formatted as relative time (e.g., "3 minutes ago", "2 days ago"), and test pass count
3. WHEN no historical snapshots exist, THE Diff_Command SHALL display an actionable message with the next command `rg snapshot` to create the first baseline
4. WHEN `--json` is passed with `--list`, THE Diff_Command SHALL output the snapshot list as a JSON array to stdout where each entry contains the fields: `ref` (index reference string), `gitCommit` (short hash), `createdAt` (ISO 8601 timestamp), and `testsPassed` (integer count)
5. WHEN `rg diff --list` completes successfully, THE Diff_Command SHALL exit with code 0

### Requirement 7: Verbose Diff Output

**User Story:** As a developer, I want detailed diff information when I need to investigate a specific change, so that I can understand exactly what happened.

#### Acceptance Criteria

1. WHEN `--verbose` is passed, THE Diff_Renderer SHALL display on stderr the full normalized schema (type-only key-value representation) for each route with schema changes, showing both the before shape from the baseline snapshot and the after shape from the comparison snapshot, each identified by route key (METHOD /path)
2. WHEN `--verbose` is passed, THE Diff_Renderer SHALL display on stderr the timing values (baseline ms, comparison ms, and delta ms) for all routes, including those within tolerance (delta ≤200ms or increase ≤50%)
3. WHEN `--verbose` is passed, THE Diff_Renderer SHALL display on stderr routes that were added (present in the after snapshot but absent from the before snapshot) or removed (present in the before snapshot but absent from the after snapshot), each identified by route key (METHOD /path)
4. WHEN `--verbose` is passed together with `--json`, THE Diff_Renderer SHALL write verbose diagnostic details to stderr only, preserving valid JSON as the sole content on stdout

### Requirement 8: Error Handling

**User Story:** As a developer, I want clear error messages when diff operations fail, so that I know exactly how to fix the problem.

#### Acceptance Criteria

1. IF no snapshot history exists and `rg diff` is invoked, THEN THE Diff_Command SHALL return an actionable error with title "No snapshot history available", cause "Only one snapshot exists — nothing to compare against", and next command "rg snapshot" to create a second baseline
2. IF the Snapshot_Index file is corrupted or contains invalid JSON, THEN THE Diff_Command SHALL return an actionable error with title "Snapshot index is unreadable", cause describing the parse failure reason, and next command "rg snapshot" to rebuild
3. IF a referenced snapshot file is missing from disk but present in the index, THEN THE Diff_Command SHALL return an actionable error with title "Snapshot file missing", cause identifying the missing file path, and next command "rg snapshot" to create a fresh baseline
4. THE Diff_Command SHALL structure every error as an Actionable error containing four fields: title (what failed), cause (why it failed), next (a copy-pasteable command to resolve it), and moreContext (a deeper debugging path such as `rg diff --help` or `rg doctor`)
5. IF `--json` mode is active when an error occurs, THEN THE Diff_Command SHALL serialize the actionable error as a JSON object to stdout with `status` set to "error" and SHALL NOT emit any ANSI escape codes or human-formatted text
