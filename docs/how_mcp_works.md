Great question. Here's how it works:

## It runs locally — no cloud needed

The MCP server runs **on the developer's machine**, right next to their code. There's no cloud hosting, no network calls to external services, no API keys.

## How the connection works

```
┌─────────────────────────────────────────────────┐
│  AI IDE (Claude Code / Cursor / Kiro)           │
│                                                 │
│  1. IDE spawns: rg mcp serve                    │
│  2. Communicates via stdin/stdout (JSON-RPC)    │
│  3. Calls tools: check, snapshot, status        │
│  4. Gets structured results back                │
└────────────────┬────────────────────────────────┘
                 │ stdin/stdout (local process)
┌────────────────▼────────────────────────────────┐
│  rg mcp serve (local process)                   │
│                                                 │
│  - Reads .regressguard/config.json              │
│  - Reads .regressguard/snapshot.json            │
│  - Runs tests locally                           │
│  - Hits localhost dev server                    │
│  - Returns JSON results                         │
└─────────────────────────────────────────────────┘
```

## Step by step

1. **User installs `rg`** on their machine (already done via curl/brew)

2. **User adds MCP config** to their IDE. For example in Kiro (`.kiro/settings/mcp.json`):
   ```json
   {
     "mcpServers": {
       "regressguard": {
         "command": "rg",
         "args": ["mcp", "serve"]
       }
     }
   }
   ```

3. **IDE spawns the process** — when the AI agent needs regression checking, the IDE starts `rg mcp serve` as a child process

4. **Communication happens over stdio** — the IDE writes JSON-RPC requests to the process's stdin, reads responses from stdout. No HTTP, no ports, no network.

5. **Agent calls tools** — after making code changes, the AI agent can call:
   - `check` → "did I break anything?"
   - `snapshot` → "save current state as the new baseline"
   - `status` → "what's the project health?"

6. **Process dies when IDE disconnects** — when stdin closes (IDE shuts down or disconnects), the MCP server exits cleanly

## Why stdio and not HTTP?

- **Zero config** — no ports to manage, no CORS, no auth tokens
- **Security** — the process only has access to the local filesystem, same as the user
- **Speed** — no network latency, direct process communication
- **Standard** — this is how all MCP servers work (same pattern as the AWS MCP servers, GitHub MCP, etc.)

## The AI agent's workflow

```
Agent: "I'll refactor the auth module"
  → makes changes to 5 files
  → calls MCP tool: check
  → gets back: { status: "critical", results: [{ route: "GET /api/auth/verify", before: 200, after: 401 }] }
  → "Oops, I broke auth. Let me fix that."
  → fixes the issue
  → calls MCP tool: check
  → gets back: { status: "pass" }
  → "All clear, safe to commit."
```

The agent never needs to know CLI syntax, parse terminal output, or deal with ANSI colors. It gets clean structured JSON through the MCP protocol.