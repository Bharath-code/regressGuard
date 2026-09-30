# End-to-End Testing Guide

How to verify RegressGuard actually works, end to end, and how to get the most
out of it day to day. Two audiences: **verifying a change to RegressGuard
itself** (steps 1-4) and **using RegressGuard well on a real project** (step 5).

## 1. Unit + integration tests (fast, run every time)

```sh
go build ./...
go test ./... -short
```

Green here means the diff engine, severity rules, snapshot integrity, and MCP
tool contracts all still hold. This is the gate — CI runs the same command.
Not a substitute for step 2: unit tests mock the HTTP layer, the fixture demo
doesn't.

## 2. Full behavioral demo against a real app (fixtures/nextjs-app)

This is the only test that drives a real dev server, real HTTP probes, and a
real regression end to end.

```sh
go build -o regressguard ./cmd/regressguard
cd fixtures/nextjs-app && npm install && cd ../..
./demo/demo.sh
```

What it proves, in order:
1. `regressguard snapshot` records a passing baseline against a live server.
2. A silent field removal (simulating an AI edit) is introduced.
3. `regressguard check` blocks with a CRITICAL schema-change finding.
4. `regressguard check --json` includes a `hint` naming the changed file.
5. Reverting the edit makes `regressguard check` pass again.

If any of those five don't happen, the core value prop is broken — fix before
anything else. `fixtures/README.md` has the manual walkthrough (open the file
yourself, edit, watch it fail) if you want to see the raw CLI output instead
of the scripted version, plus an auth-regression variant (`/api/auth/verify`,
200→401) that `demo.sh` doesn't cover.

## 3. Pre-commit hook path

The hook is a separate code path from `regressguard check` (it shells out with an
absolute binary path) — test it directly,
don't assume `regressguard check` passing means the hook works.

```sh
cd fixtures/nextjs-app
regressguard hook install
git add . && git commit -m "test regression"   # should block if check fails
regressguard hook uninstall
```

## 4. MCP path (the primary interface — don't skip this)

`internal/mcprun` has unit tests, but they exercise the tool handlers
directly. Confirm the stdio transport actually works with a real client:

```sh
regressguard mcp serve
```

```sh
claude mcp add regressguard -- /full/path/to/regressguard mcp serve
```

Then run `regressguard snapshot` yourself (agents get no `snapshot` tool by default).
In a Claude Code session against `fixtures/nextjs-app`, break something, ask
it to call `check`, and confirm the JSON payload
matches `docs/json-contract.md`. Check `.regressguard/audit.log` (or
equivalent) got an entry per call.

## 5. Using RegressGuard well on a real project (not just testing it)

- **Snapshot before the agent touches anything, not after.** The baseline is
  only "known-good" if it was captured before the edit. Get in the habit of
  `regressguard snapshot` as the first command in a session, not a cleanup step.
- **Keep the dev server running during snapshot and check.** Both need to hit
  live routes; a stopped server means routes get skipped (or, since the
  zero-route fix, an explicit warning) instead of actually verified.
- **Wire the MCP server into the agent's loop, not just the CLI.** The whole
  differentiator is the agent catching its own regression before you see the
  diff — `regressguard check` run by a human after the fact is the fallback, not the
  primary mode.
- **Install the git hook as a backstop**, not the main check — it's the
  last line of defense for edits made outside an agent loop.
- **Watch for the "0 captured" / "0 in snapshot" warning.** It means no
  routes are configured or the stack isn't auto-detected (Python/Django/
  FastAPI aren't supported yet) — RegressGuard is running but not actually
  protecting anything. Add routes to `.regressguard/config.json` or file it
  as a gap, don't silently trust a "pass".
- **Run `regressguard doctor` after any environment change** (new machine,
  Node/Go upgrade) — it catches stale hooks
  before they cause a false sense of safety.
