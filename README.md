# RegressGuard

**Before you commit, know what broke.**

RegressGuard is a CLI safety net for developers using AI coding agents (Claude Code, Cursor, Codex). It catches silent regressions introduced during AI sessions — before they reach production.

Two commands. Zero test-writing required. Under 15 seconds.

```
rg snapshot   # record the known-good state
rg check      # compare after AI edits — see what broke
```

---

## Install

**macOS / Linux (recommended)**

```sh
curl -fsSL https://raw.githubusercontent.com/Bharath-code/regressguard/main/install.sh | sh
```

**Homebrew**

```sh
brew install Bharath-code/tap/rg
```

**Verify**

```sh
rg version
```

---

## Quickstart (3 minutes)

### 1. Initialize your project

```sh
cd your-project
rg init
```

RegressGuard detects your test command, framework, and dev server URL automatically.

### 2. Record the baseline before your AI session

Make sure your dev server is running, then:

```sh
rg snapshot
```

Output:

```
Snapshot

OK Tests       42 passed, 0 failed       6.8s
OK Routes      6 captured, 2 skipped
OK Schemas     6 hashed

Saved:
  .regressguard/snapshot.json

Next:
  Ask your AI agent to make the code change, then run:
  rg check
```

### 3. Run your AI agent

Let Claude Code, Cursor, or Codex make its changes.

### 4. Check for regressions before committing

```sh
rg check
```

**Clean — safe to commit:**

```
Check

OK No regressions detected

  Tests       42 passed, 0 failed
  Routes      6 unchanged
  Timing      within tolerance

Safe to commit.
```

**Regression found — commit blocked:**

```
Check

X 2 regressions detected

  Route                                 Before    After     Change
  GET /api/users                        schema    schema    schema
    - role (string, removed)
    + age (number, added)
  POST /api/user/update                 200       500       status

Likely cause:
  Auth/session behavior or routing changed during the last code edit.

Changed files since snapshot:
  app/api/users/route.ts
  internal/auth/session.go

Next:
  rg check --verbose
  git diff

Commit blocked.
```

Exit code `1` on critical — works with git hooks and CI.

---

## Git Hook (auto-protect every commit)

```sh
rg hook install
```

Now `rg check` runs automatically before every `git commit`. When a critical regression is detected, the commit is blocked with a compact output:

```
RegressGuard pre-commit

X 1 regression detected
  POST /api/user/update status changed from 200 to 500

Run:
  rg check --verbose

Commit blocked. Use --no-verify only if you accept the risk.
```

Bypass with `git commit --no-verify` only when you accept the risk.

---

## Commands

| Command | Purpose |
|---|---|
| `rg init` | Configure RegressGuard for this project |
| `rg snapshot` | Record the current passing state |
| `rg check` | Compare current state against the snapshot |
| `rg hook install` | Install the pre-commit git hook |
| `rg hook uninstall` | Remove the git hook |
| `rg config get <key>` | Read a config value |
| `rg config set <key> <value>` | Write a config value |
| `rg doctor` | Diagnose setup issues |
| `rg completion <shell>` | Generate shell autocompletions (bash, zsh, fish) |
| `rg version` | Print version and build metadata |

Run `rg <command> --help` for flags, examples, and exit codes.

---

## Configuration

Config lives in `.regressguard/config.json` (human-readable, git-ignoreable).

```json
{
  "version": 1,
  "testCommand": "npm test",
  "serverUrl": "http://localhost:3000",
  "auth": {
    "mode": "bearer",
    "testToken": "your-test-token",
    "headerName": "Authorization",
    "prefix": "Bearer"
  },
  "ignoreFields": ["requestId", "traceId"],
  "routes": [
    { "method": "GET", "path": "/api/health" },
    { "method": "GET", "path": "/api/users" },
    { "method": "GET", "path": "/api/admin", "skip": true }
  ]
}
```

**Auth modes:** `bearer` (Authorization header), `cookie` (Cookie header), or omit for public routes only.

**ignoreFields:** Fields to exclude from schema comparison — useful for volatile app-specific values like `requestId` or `traceId`.

---

## How it works

1. `rg snapshot` runs your test suite and hits each configured route. It records pass/fail counts, HTTP status codes, and a normalized schema hash for each response.

2. `rg check` reruns the same tests and routes, then diffs against the snapshot:
   - **CRITICAL**: test suite newly failing, status code changed, response schema changed (e.g. field removed/added/changed)
   - **WARNING**: response time increased >200ms and >50% of baseline
   - **PASS**: everything within acceptable variance

3. Schema comparison automatically normalizes JSON payloads:
   - **Default Dynamic Keys**: Strips 16 common dynamic keys (`id`, `uuid`, `token`, `nonce`, `timestamp`, `createdAt`, `updatedAt`, `deletedAt`, `created_at`, `updated_at`, `deleted_at`, `sessionId`, `accessToken`, `refreshToken`, `expiresAt`, `expires_at`) before hashing.
   - **Pattern Detection**: Automatically detects ISO-8601 date strings, UUIDs, and JWTs, replacing them with generic type representations (`"date"`, `"uuid"`, `"token"`).
   - **User Customization**: Respects custom `ignoreFields` defined in config.

   This ensures the shape integrity of endpoints remains stable across runs even when database IDs and timestamps change.

4. A route whose only change is a non-blocking **WARNING** (e.g. a timing regression) is reported on its own line and is **not** counted in the "Routes: N unchanged" summary or in `summary.passed` of `--json` output.

### Known limitations

These are deliberate trade-offs in v1 — favoring zero false positives over exhaustive detection. They are on the roadmap, not accidental:

- **Test results are compared by count, not by identity.** `rg check` flags a CRITICAL only when the number of failing tests *increases*. If one test starts failing while a previously-failing test starts passing (net failure count unchanged), the regression is not detected. Pair `rg check` with your normal test runner in CI for per-test assertions.
- **Array schemas are inferred from the first element.** The schema normalizer represents a JSON array's shape using its first element. If later elements have a different shape (heterogeneous arrays), that divergence is not reflected in the schema hash and will not be flagged.

---

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Pass or warnings only — safe to commit |
| `1` | Critical regression detected — commit blocked |
| `2` | Usage, config, or runtime error |

---

## Scripting and CI

```sh
# JSON output for scripts and agents
rg check --json | jq .status

# Verbose diagnostics on stderr (stdout stays clean JSON)
rg check --json --verbose

# Disable color for CI
NO_COLOR=1 rg check
```

---

## Supported stacks (v1)

- **Frameworks**: Next.js App Router, Express, Hono
- **Test runners**: Vitest, Jest, Bun test, npm test
- **Package managers**: npm, pnpm, yarn, bun
- **Auth**: Bearer token, Cookie header, public routes

Python, FastAPI, and Django support is planned for v2.

---

## Demo fixture

A minimal Next.js API fixture is included in `fixtures/nextjs-app` for demos and testing. See [fixtures/README.md](fixtures/README.md).

---

## License

MIT — see [LICENSE](LICENSE).

---

*From the same developer as [git-scope](https://github.com/Bharath-code/git-scope).*
