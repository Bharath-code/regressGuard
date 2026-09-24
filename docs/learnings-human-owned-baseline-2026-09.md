# Learnings: human-owned baseline over MCP (E13-T9, 2026-09-25)

## Why it was needed
RegressGuard's promise is that an agent can't report "done" while it has
changed an API contract nobody approved. The MCP server exposed `snapshot`,
so an agent could close that gap by itself: `check` fails, then `snapshot`,
then `check` passes. The tool description even said "Creates a baseline", which
invited it. A guard that the guarded party can reset is not a guard.

## What changed
- `rg mcp serve` registers `check` and `status` always. It registers
  `snapshot` only when `.regressguard/config.json` has
  `"mcp": {"allowSnapshot": true}`. Config is read once at startup.
- The `check` tool description tells agents: fix the code, or if the change
  is intentional, ask the user to run `rg snapshot`. Don't re-record it.
- Tests: `TestNewServer_snapshotHiddenByDefault`, `...HiddenWithoutConfig`,
  `...ExposedWhenOptedIn`. Also verified with a real `tools/list` over stdio.

## How it's used
- Human: run `rg snapshot` when the app is known-good, and again to accept an
  intentional contract change.
- Agent: calls `check` in its loop and fixes regressions. For intentional
  changes it hands the decision back to the human.
- Opting in (solo projects that trust their agent) is one config line.

## Known limit and next step
An agent with shell access can still run `rg snapshot` directly. MCP gating
removes the invited path, not every path. The next layer is making a re-recorded
baseline visible. Options: the git hook or `check` flags a `snapshot.json` that
changed in the same uncommitted work as the code, or CI treats baseline changes
as a separate approval, like reviewing a snapshot-test update.
