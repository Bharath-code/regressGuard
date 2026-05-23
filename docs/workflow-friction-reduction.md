# Workflow & Friction Reduction (Section 19.2)

## What We Built

Five workflow improvements that reduce friction between installing RegressGuard and having it become part of the developer's muscle memory.

## Features Implemented

### W1: Zero-Config First Run (`rg quickstart`)

**Problem:** New users had to run `rg init` then `rg snapshot` as separate steps. If they forgot the server URL flag in non-interactive mode, they'd get an error.

**Solution:** `rg quickstart` chains init + snapshot in one command. It auto-detects everything (framework, test command, routes) and takes a snapshot immediately.

**Usage:**
```bash
# In a Next.js/Express project with dev server running:
rg quickstart

# With explicit server URL:
rg quickstart --server-url http://localhost:3000
```

**How it works:** Calls `initrun.Run()` with `--yes` (non-interactive, auto-detect) followed by `snapshotrun.Run()`. If init fails (e.g., server not reachable), it surfaces the actionable error immediately.

---

### W2: Git Hook Auto-Install on Init

**Problem:** After `rg init`, users had to remember to run `rg hook install` separately. Many never did, missing the core protection.

**Solution:** In interactive mode, `rg init` now prompts "Install pre-commit hook?" after writing the config. If the user accepts, the hook is installed immediately.

**Behavior:**
- Only shown in interactive mode (TTY with no `--yes` flag)
- Only shown when `.git` directory exists
- Only shown when hook isn't already installed
- Uses the same `huh` confirm widget as other init prompts
- Non-interactive mode prints a suggestion line instead

---

### W3: Snapshot Auto-Refresh

**Problem:** Users who ran `rg check` daily would get "Snapshot is 3d old" warnings repeatedly, even when everything was passing. This trained them to ignore the warning.

**Solution:** When `rg check` passes AND the snapshot is older than 24 hours, the snapshot is silently refreshed using the current check results. A subtle note appears on stderr.

**Behavior:**
- Only triggers on `pass` status (not warning or critical)
- Only triggers when snapshot age > 24h
- Prints `i Snapshot auto-refreshed (was stale).` on stderr
- Does not affect exit code or stdout output
- Does not re-run tests (uses the results already computed during check)

---

### W5: Route Discovery from Test Files

**Problem:** Static route discovery (scanning `app/api/` or Express patterns) misses routes that are only referenced in test files. Users had to manually add these to config.

**Solution:** `rg init` now also scans test files (`.test.ts`, `.spec.ts`, etc.) for route assertions like `fetch("/api/users")` or `.get("/api/health")`. Discovered routes are merged with statically-discovered ones.

**Patterns matched:**
- `fetch("/api/...")` → GET
- `.get("/api/...")` → GET
- `.post("/api/...")` → POST
- `.put("/api/...")` → PUT
- `.delete("/api/...")` → DELETE
- `request(app).get("/api/...")` → GET

**Behavior:**
- Only matches routes starting with `/api` (avoids false positives)
- Skips dynamic template literals (`${baseUrl}/api/...`)
- Deduplicates against statically-discovered routes
- Scans `tests/`, `test/`, `__tests__/`, `src/`, `spec/` directories

---

### W6: `rg snapshot --accept`

**Problem:** After an intentional schema change, users had to run a full `rg snapshot` (which re-runs the entire test suite) just to accept the new route responses. This was slow and unnecessary.

**Solution:** `rg snapshot --accept` re-hits only routes and updates the existing snapshot without re-running tests. It's 2-5x faster than a full snapshot.

**Usage:**
```bash
# After rg check shows an intentional change:
rg snapshot --accept
```

**How it works:**
1. Loads the existing snapshot (fails if none exists)
2. Probes server reachability
3. Re-hits all configured routes
4. Updates route records in the existing snapshot
5. Preserves test results from the previous snapshot
6. Updates timestamp and git commit
7. Archives to history

**When to use:**
- After `rg check` reports a CRITICAL that you know is intentional
- When you only changed API responses (not test behavior)
- When you want a faster alternative to full `rg snapshot`

---

## Files Changed

| File | Change |
|------|--------|
| `internal/cli/cli.go` | Added `quickstart` command, `--accept` flag on snapshot |
| `internal/initrun/initrun.go` | Added `offerHookInstall()` for W2 |
| `internal/checkrun/checkrun.go` | Added `autoRefreshSnapshot()` for W3 |
| `internal/snapshotrun/snapshotrun.go` | Added `Accept` option and `runAccept()` for W6 |
| `internal/scanner/scanner.go` | Added `DiscoverRoutesFromTests()` for W5 |

## Items Not Implemented (by design)

| ID | Item | Reason |
|----|------|--------|
| W4 | IDE extension | P2, requires separate VS Code extension project |
| W7 | npx wrapper | P2, requires npm package publishing infrastructure |
| W8 | Monorepo support | P2/v2, complex scope beyond current architecture |
