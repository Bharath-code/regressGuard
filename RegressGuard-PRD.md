**PRODUCT REQUIREMENTS DOCUMENT**

**RegressGuard**

The Safety Net for AI-Generated Code

| Version | v1.0 — MVP |
| :---- | :---- |
| **Author** | Bharath |
| **Date** | May 2026 |
| **Status** | Spec Ready — Implementation Not Started |
| **Target Launch** | 7 Days from Day 1 |

# **1\. Executive Summary**

RegressGuard is a CLI-first safety net for solo developers and small teams using AI coding agents (Claude Code, Cursor, Codex, Windsurf). It detects unintentional regressions introduced during AI coding sessions — before they reach production.

Two commands. Zero test-writing required. Catches the majority of accidental breakages in under 15 seconds.

**The Core Value Proposition:**

| "Before you commit, know what broke." — RegressGuard detects silent regressions introduced by AI coding agents before they reach production. |
| :---- |

# **2\. Problem Statement**

## **2.1 The Core Problem**

AI coding agents are rewriting how software gets built. Claude Code authored 4% of all public GitHub commits in March 2026\. Codex hit 3 million weekly active users in April 2026\. This adoption is accelerating at 50% month-over-month.

But speed without safety creates a new category of failure: silent regressions. The AI fixes one thing and quietly breaks another. The developer ships it. The user finds it Tuesday.

## **2.2 The Data**

| Stat 1 | Stat 2 | Stat 3 | Stat 4 |
| :---- | :---- | :---- | :---- |
| 43% | 242.7% | 31.3% | \~3 hrs |
| of AI changes need production debugging | increase in incidents per PR since AI adoption | more PRs merged without review | average time lost per silent regression |

## **2.3 Why Existing Tools Fail**

| Tool | What it Does | The Gap |
| :---- | :---- | :---- |
| Claude Code | Runs tests if you tell it to | No before/after baseline comparison |
| Cursor | None — confirmed silent reverts in March 2026 | Opposite of protection |
| GitHub Actions | CI regression detection | Runs after push — too late |
| Windsurf | Lint only | No behavior comparison |
| Braintrust/Autonoma | Enterprise evaluation platforms | Complex setup, team-focused, expensive |

| The gap: nobody owns pre-commit, post-AI-session regression detection for solo developers. That is RegressGuard. |
| :---- |

# **3\. Target Users & ICP**

## **3.1 Primary ICP — v1**

| Who | Solo developers and small teams (2-5 devs) actively using Claude Code, Cursor, or Codex |
| :---- | :---- |
| **Billing Model** | API billing (not flat subscription) — they feel every token spent |
| **Experience** | Been burned by a silent regression at least once |
| **Stack** | Go CLI targeting Node.js / Bun / TypeScript projects — Next.js, Express, Hono, Vitest, Jest |
| **Psychographic** | Not cheap — uncertain. They will spend money if they know what they get |
| **Location** | Global — India-first for marketing, English-first for product |

## **3.2 Secondary ICP — v2+**

| Engineering Leads | Need visibility into AI activity across their team's codebase |
| :---- | :---- |
| **Open Source Maintainers** | Accept AI-generated PRs and need automated safety checks |
| **Freelance Devs** | Cannot afford production incidents on client projects |

## **3.3 Market Size**

| Segment | Size | Note |
| :---- | :---- | :---- |
| AI Coding Agent Users (2026) | \~10M developers globally | Growing 50% MoM |
| Experience Regressions Regularly | \~2M (20% of above) | Primary addressable market |
| Willing to Pay $9/month | \~40,000 (2% conversion) | Conservative TAM at scale |
| 12-Month Realistic Target | 500-1,000 users | ₹4.5L-₹9L MRR |

# **4\. User Stories & Real Scenarios**

## **4.1 The Silent Auth Regression**

| Scenario: Priya asks Claude Code to refactor the auth module. Claude touches 11 files. Everything looks fine. She commits. Tuesday: users cannot log in. |
| :---- |

Without RegressGuard: 3-hour debugging session to trace a renamed variable in a utility function that two auth middleware modules depended on.

With RegressGuard:

1. rg snapshot — 8 seconds. Records 42 passing tests \+ 6 route responses.

2. Claude Code runs. Touches 11 files.

3. rg check — 12 seconds.

  ✓ 42 tests unchanged                    

  ✗ GET /api/auth/verify  200 → 401        

  ✗ POST /api/user/update  200 → 500       

  2 regressions detected. Commit blocked.


Caught in 12 seconds. Not Tuesday.

## **4.2 The Schema Silent Change**

| Scenario: Arjun adds a feature to his SaaS. The AI refactors the user profile endpoint. Response still returns 200 but removes the "subscription" field his frontend depends on. |
| :---- |

With RegressGuard: schema hash mismatch detected immediately. Warning: "response shape changed — subscription field removed." He checks. He fixes. He ships correctly.

## **4.3 The Performance Regression**

| Scenario: An AI refactor introduces an N+1 query in the auth middleware. Tests pass. Status codes are correct. But auth now takes 420ms instead of 40ms. |
| :---- |

With RegressGuard: timing comparison flags the degradation as a warning. Not a blocker — but visible before production.

## **4.4 The Open Source Maintainer**

Ravi maintains a 1,400-star library. He accepts an AI-generated PR from a contributor. rg check runs automatically via git hook. Catches that a previously passing test suite now fails on Node 18\. He requests a fix before merging.

# **5\. Product Features & Acceptance Criteria**

## **5.1 v1 MVP Features (Week 1\)**

Ruthlessly minimal. Only what is needed to catch regressions for real projects.

### **Feature 1: rg init**

| Command | rg init |
| :---- | :---- |
| **Purpose** | Auto-discover project ecosystem, generate .regressguard/config.json |
| **AC 1** | Detects package.json, bun.lock, pyproject.toml automatically |
| **AC 2** | Identifies test command (vitest, jest, bun test, pytest) |
| **AC 3** | Discovers API routes in Next.js App Router and Express statically |
| **AC 4** | Generates valid config.json in under 3 seconds |
| **AC 5** | Asks user for dev server URL if not auto-detected (localhost:3000 default) |
| **AC 6** | Works on a fresh project with zero prior configuration |
| **AC 7** | Detects TTY vs non-TTY: uses guided prompts for humans, never hangs in scripts/agents |
| **AC 8** | Non-interactive mode prints the required next command or flag instead of prompting |

### **Feature 2: rg snapshot**

| Command | rg snapshot |
| :---- | :---- |
| **Purpose** | Record current passing state — tests, routes, response schemas |
| **AC 1** | Runs test suite and records pass/fail counts — not individual test names |
| **AC 2** | Hits discovered routes and records status codes \+ normalized schema hashes |
| **AC 3** | Records response timing per route |
| **AC 4** | Completes in under 15 seconds for projects with \<50 routes |
| **AC 5** | Stores snapshot to .regressguard/snapshot.json — human readable |
| **AC 6** | Handles auth via config-defined test token (Authorization header) |
| **AC 7** | Gracefully skips routes that require body params it cannot infer |
| **AC 8** | Supports --json for clean machine-readable output on stdout |
| **AC 9** | Progress, warnings, and verbose details go to stderr, never stdout |

### **Feature 3: rg check**

| Command | rg check |
| :---- | :---- |
| **Purpose** | Compare current state against snapshot, report regressions |
| **AC 1** | Reruns same test suite — reports delta vs snapshot |
| **AC 2** | Hits same routes — compares status codes, schema hashes, timing |
| **AC 3** | Outputs CRITICAL / WARNING / PASS per check item |
| **AC 4** | Exits with code 1 on any CRITICAL finding (blockable in git hooks) |
| **AC 5** | Exits with code 0 on WARNING-only (non-blocking by default) |
| **AC 6** | Total runtime under 20 seconds for standard projects |
| **AC 7** | Output is human-readable and copy-pasteable for bug reports |
| **AC 8** | False positive rate under 5% on standard Next.js/Express projects |
| **AC 9** | Supports --json for structured results and stable agent/script parsing |
| **AC 10** | Supports --verbose for route-level request/response metadata on stderr |
| **AC 11** | All failures include what broke, likely cause, and a copy-pasteable next action |

### **Feature 4: Git Hook Integration**

| Command | rg hook install |
| :---- | :---- |
| **Purpose** | Auto-run rg check on every git commit attempt |
| **AC 1** | Installs pre-commit hook in .git/hooks/ |
| **AC 2** | Blocks commit if CRITICAL regressions found |
| **AC 3** | Allows \--no-verify bypass for emergencies |
| **AC 4** | Works with husky and lint-staged setups |
| **AC 5** | Hook output is short by default and suggests rg check --verbose for deeper diagnostics |

### **Feature 5: CLI Foundation**

| Command | rg --help, rg version, rg config, rg doctor |
| :---- | :---- |
| **Purpose** | Make the CLI self-documenting, scriptable, and easy to debug |
| **AC 1** | rg --help shows compact top-level command groups only |
| **AC 2** | rg <command> --help shows flags, examples, exit codes, and agent-friendly guidance |
| **AC 3** | rg config get/set supports common fields like serverUrl, testCommand, auth.testToken, ignoreFields |
| **AC 4** | rg doctor verifies config, snapshot presence, test command availability, and dev server reachability |
| **AC 5** | rg version prints version, commit, build date, OS, and architecture |
| **AC 6** | Shell completions are available for zsh, bash, and fish |
| **AC 7** | CLI screens follow the RegressGuard terminal design system for colors, spacing, symbols, and next-step layout |
| **AC 8** | Default happy-path screens fit in one terminal viewport at 80 columns |

## **5.2 v2 Features (Month 2-3)**

| Session History | Track which AI sessions introduced which regressions over time |
| :---- | :---- |
| **AI Attribution** | Show which files AI touched per session, correlation with regressions |
| **Web Dashboard** | Per-project history, trends, regression frequency per file |
| **Team Snapshots** | Shared snapshot store for multi-developer teams |
| **Python Support** | pytest, FastAPI, Django route discovery |

## **5.3 v3 Features (Month 4-6)**

| Risk Scoring | Per-file risk score based on historical regression frequency |
| :---- | :---- |
| **PR Integration** | GitHub Action that runs rg check on every pull request |
| **Compliance Reports** | PDF audit trail of AI-generated code changes for regulated industries |
| **Multi-agent Support** | Track sessions across Cursor, Claude Code, Codex simultaneously |

# **6\. CLI UX Principles**

RegressGuard is used by both humans and AI coding agents. The CLI must be easy to explore step-by-step, safe to script, and impossible to confuse with noisy output.

## **6.1 Progressive Disclosure**

| Principle | Requirement |
| :---- | :---- |
| Top-level help stays small | rg --help shows only high-level command groups: init, snapshot, check, hook, config, version |
| Drill-down is exhaustive | rg check --help shows all check-specific flags, examples, exit codes, and JSON schema notes |
| Noun-verb structure | Prefer rg hook install, rg config set, rg snapshot create over ambiguous action soup |
| Agent-friendly discovery | Help text explicitly says agents should prefer --help over hardcoded command knowledge |
| No context bloat | Default output avoids dumping full route lists, raw responses, or config unless requested |

## **6.2 Actionable Errors**

Every error must include:

1. What went wrong.
2. The likely cause.
3. A copy-pasteable next action.
4. A deeper debugging path, usually --verbose or --help.

Examples:

Missing snapshot:

rg check failed: no snapshot found.

Run this first:

  rg snapshot

Need more context:

  rg check --help

Route returned 401:

GET /api/auth/verify returned 401, but the snapshot expected 200.

Likely cause: this route needs auth, and no test token/cookie is configured.

Try:

  rg config set auth.testToken <TOKEN>

Then rerun:

  rg snapshot

## **6.3 Data vs Messages**

| Stream | Allowed Content |
| :---- | :---- |
| stdout | Final human report by default, or clean JSON only when --json is used |
| stderr | Progress, warnings, verbose logs, request metadata, troubleshooting hints |

Rules:

1. When --json is used, stdout must contain valid JSON and nothing else.
2. ANSI colors are disabled automatically when stdout is not a TTY.
3. --verbose writes diagnostics to stderr so rg check --json \| jq remains safe.
4. Exit codes carry automation meaning: 0 pass/warnings only, 1 critical regression, 2 usage/config/runtime error.

## **6.4 Interactive vs Non-Interactive**

RegressGuard detects whether stdin/stdout are attached to a TTY.

| Mode | Behavior |
| :---- | :---- |
| Interactive human | rg init can use Bubble Tea/huh prompts, selections, confirmation, and friendly formatting |
| Non-interactive agent/script | No prompts, no hanging flows, no required keyboard interaction |
| Missing input | Print the exact flag or command needed to continue |
| CI/git hook | Short output by default, with --verbose suggested for diagnosis |

## **6.5 Terminal Design System**

RegressGuard should feel like a serious infrastructure tool: calm, precise, fast, and trustworthy. The visual language is closer to GitHub CLI, Vercel CLI, Stripe CLI, pnpm, and Charm tools than to a playful full-screen app.

### **Design Goals**

| Goal | Rule |
| :---- | :---- |
| Reduce friction | Every screen must answer: what happened, what matters, what should I run next |
| Preserve trust | Never hide critical detail behind decoration; design clarifies state |
| Respect agents | Human styling must collapse cleanly into structured output and short help |
| Reward good flow | Successful setup/checks should feel fast and decisive, not chatty |
| Avoid CLI fatigue | No walls of text, no noisy banners, no forced full-screen TUI |

### **Typography**

| Token | Terminal Rendering | Usage |
| :---- | :---- | :---- |
| Font family | User terminal default; recommend Geist Mono, JetBrains Mono, SF Mono, or IBM Plex Mono in docs/screenshots | All CLI output |
| Brand wordmark | Bold monospace text, no ASCII art banner by default | rg --help, rg init header |
| Section label | Bold, Title Case, max 24 chars | Summary, Regressions, Next Steps |
| Body | Regular monospace | Descriptions and hints |
| Data/code | Monospace, dim or neutral | Routes, commands, paths, hashes |
| Numbers | Monospace tabular alignment | Counts, timings, deltas |

### **Color Tokens**

Colors must work in dark and light terminals. Use semantic ANSI colors only, with no gradients, glow, or oversaturated effects.

| Token | ANSI | Hex Reference | Usage |
| :---- | :---- | :---- | :---- |
| rg.ok | Green | #2DA44E | Pass, installed, completed |
| rg.warn | Yellow | #B88700 | Warning, skipped route, timing risk |
| rg.fail | Red | #CF222E | Critical regression, blocked commit |
| rg.info | Cyan/Blue | #0969DA | Links, commands, active step |
| rg.muted | Bright Black/Gray | #6E7781 | Metadata, paths, secondary hints |
| rg.text | Default foreground | terminal default | Primary copy |
| rg.border | Dim gray | #8C959F | Tables, separators |

Rules:

1. Respect NO_COLOR, FORCE_COLOR, TERM=dumb, and non-TTY output.
2. Use color plus text labels, never color alone.
3. Keep one accent at a time. A critical screen should not also contain blue/yellow decoration.
4. Do not print emoji. Use ASCII symbols and text labels.

### **Symbols**

| Meaning | Symbol | Text Label |
| :---- | :---- | :---- |
| Pass | OK | PASS |
| Warning | ! | WARNING |
| Critical | X | CRITICAL |
| Info | i | INFO |
| Skipped | - | SKIPPED |
| Running | > | RUNNING |

### **Spacing & Layout**

| Pattern | Rule |
| :---- | :---- |
| Max width | Keep human output readable at 80-100 columns |
| Summary first | Always show the outcome before details |
| One blank line | Separate major sections with one blank line only |
| Tables | Align columns, truncate long paths/routes intelligently |
| Next steps | End failure states with one or two copy-pasteable commands |
| Default verbosity | Show top 3-5 most important findings; suggest --verbose for full detail |

### **Motion & Progress**

| Context | Behavior |
| :---- | :---- |
| Interactive TTY | Use spinner/progress only for operations over 400ms |
| Non-TTY/CI | No spinner frames; print stable line-based events only when verbose |
| Long checks | Show current phase: tests, routes, diff |
| Completion | Replace spinner with final stable result line |

## **6.6 Core User Flow**

The default human journey is four steps:

1. Install RegressGuard.
2. Run rg init once per project.
3. Run rg snapshot before an AI coding session.
4. Run rg check after the AI coding session and before commit.

The CLI must make this path obvious without requiring docs.

### **Flow A: First Install**

Command:

  curl -fsSL https://regressguard.dev/install.sh \| sh

Screen:

  Installing RegressGuard
  OK Installed rg 0.1.0 to /usr/local/bin/rg

  Verify:
    rg version

Success criteria:

1. Install output is under 8 lines.
2. Verification command is visible.
3. If PATH is missing, output shows the exact export command.

### **Flow B: First Run / Help**

Command:

  rg

Screen:

  RegressGuard
  Before you commit, know what broke.

  Commands:
    init       Configure RegressGuard for this project
    snapshot   Record the current passing state
    check      Compare current state against the snapshot
    hook       Install or remove git hooks
    config     View or edit project config
    doctor     Diagnose setup issues

  Start:
    rg init

Success criteria:

1. No more than six command groups at top level.
2. No flag dump at top level.
3. Clear first command.

### **Flow C: Guided Init**

Command:

  rg init

Interactive TTY screen sequence:

  RegressGuard init

  OK Found package.json
  OK Detected Next.js App Router
  OK Detected test command: npm test
  ! Dev server not running at http://localhost:3000

  Select dev server URL
    http://localhost:3000
    http://localhost:5173
    Enter custom URL

  Configure auth?
    Public routes only
    Bearer token
    Cookie header

  OK Wrote .regressguard/config.json

  Next:
    rg snapshot

Non-interactive behavior:

  rg init failed: dev server URL is required in non-interactive mode.

  Run:
    rg init --server-url http://localhost:3000

Success criteria:

1. Human init feels guided and finite.
2. Agent/script mode never waits for input.
3. Final line always gives the next command.

### **Flow D: Snapshot**

Command:

  rg snapshot

Screen:

  Snapshot

  OK Tests       42 passed, 0 failed       6.8s
  OK Routes      6 captured, 2 skipped     3.1s
  OK Schemas     6 hashed

  Saved:
    .regressguard/snapshot.json

  Next:
    Ask your AI agent to make the code change, then run:
    rg check

Success criteria:

1. User sees what baseline was captured.
2. Skipped routes are visible but not scary.
3. The next action is explicit.

### **Flow E: Clean Check**

Command:

  rg check

Screen:

  Check

  PASS No regressions detected

  Tests       42 passed, 0 failed
  Routes      6 unchanged
  Timing      within tolerance

  Safe to commit.

Success criteria:

1. Outcome appears in first three lines.
2. No unnecessary detail.
3. Runtime and route details available with --verbose.

### **Flow F: Regression Found**

Command:

  rg check

Screen:

  Check

  CRITICAL 2 regressions detected

  Route                    Before   After   Change
  GET /api/auth/verify     200      401     status
  POST /api/user/update    200      500     status

  Likely cause:
    Auth/session behavior changed during the last code edit.

  Next:
    rg check --verbose
    git diff

  Commit blocked.

Success criteria:

1. Failure is unmistakable but calm.
2. Table shows only decision-critical data by default.
3. Next steps are copy-pasteable.
4. Exit code is 1.

### **Flow G: Warning Only**

Command:

  rg check

Screen:

  Check

  WARNING 1 non-blocking change

  Route                 Change
  GET /api/profile      +248ms slower

  Next:
    rg check --verbose

  Commit allowed.

Success criteria:

1. Warnings do not feel like failures.
2. User knows commit is allowed.
3. Exit code is 0.

### **Flow H: JSON / Agent Mode**

Command:

  rg check --json

stdout:

  {
    "status": "critical",
    "summary": {
      "critical": 2,
      "warnings": 0,
      "passed": 48
    },
    "results": []
  }

stderr:

  INFO Run rg check --verbose for request metadata.

Success criteria:

1. stdout is parseable JSON and nothing else.
2. stderr can contain hints and diagnostics.
3. Schema is stable across minor versions.

### **Flow I: Git Hook**

Command:

  git commit -m "refactor auth"

Screen:

  RegressGuard pre-commit

  CRITICAL 2 regressions detected
  Run:
    rg check --verbose

  Commit blocked. Use --no-verify only if you accept the risk.

Success criteria:

1. Hook output is shorter than normal rg check.
2. It never opens an interactive prompt.
3. It clearly explains the bypass without encouraging it.

## **6.7 Screen Inventory**

| Screen | Trigger | Primary Job | Must Show | Must Not Show |
| :---- | :---- | :---- | :---- | :---- |
| Top-level help | rg, rg --help | Orient | Command groups, start command | Full flag list |
| Command help | rg check --help | Teach | Usage, examples, flags, exit codes | Unrelated commands |
| Init detect | rg init | Configure | Detected stack, test command, server URL, auth mode | Raw config dump |
| Snapshot running | rg snapshot | Reassure | Phase progress | Raw response bodies |
| Snapshot complete | rg snapshot | Baseline confidence | tests/routes/schemas saved, next command | Long route list by default |
| Check running | rg check | Reassure | Phase progress | Spinner in non-TTY |
| Check pass | rg check | Confirm safety | PASS, compact counts | Verbose route metadata |
| Check warning | rg check | Inform, do not block | WARNING, non-blocking status, next command | Red/critical styling |
| Check critical | rg check | Stop bad commit | CRITICAL, top findings, likely cause, next commands | Blame language |
| Doctor | rg doctor | Diagnose | config/server/test/snapshot status | Irrelevant marketing copy |
| Config | rg config get/set | Edit safely | key/value, file path, next command | Secrets in plain text unless explicitly requested |
| Hook install | rg hook install | Confirm protection | hook path, behavior, uninstall command | Overlong explanation |

## **6.8 Quality Bar**

Before launch, capture terminal screenshots or recordings for:

1. rg --help at 80 columns.
2. rg init interactive success.
3. rg init non-interactive missing input.
4. rg snapshot success.
5. rg check pass.
6. rg check warning.
7. rg check critical.
8. rg check --json piped into jq.
9. git hook blocked commit.
10. NO_COLOR=1 rg check.

Acceptance:

1. No line wraps awkwardly at 80 columns.
2. No screen exceeds one viewport for the default happy path.
3. Every failure ends with a next command.
4. JSON mode stays valid under --verbose.
5. Non-interactive mode never waits for input.

# **7\. Technology Stack**

## **7.1 Stack Decisions**

| Layer | Choice | Reason | Why This |
| :---- | :---- | :---- | :---- |
| Runtime | Go 1.22+ | Fast startup, single static binary, excellent subprocess/filesystem support | Serious devtool feel \+ pre-commit speed |
| CLI Framework | Cobra or urfave/cli | Stable command parsing, flags, shell completion, scriptable behavior | Mature CLI foundation |
| Interactive Init | Bubble Tea / Charm huh \+ Lip Gloss | Polished guided onboarding for rg init only | Great DX without making checks TUI-dependent |
| Language | Go | Type safety, simple concurrency, cross-platform binaries | Reliability \+ low install friction |
| TTY Detection | golang.org/x/term or equivalent isatty check | Branch cleanly between human prompts and script/agent behavior | Prevents hanging automation |
| HTTP Testing | Go net/http | Standard library, no runtime dependency | Minimal footprint |
| Schema Hashing | crypto/sha256 \+ custom normalizer | Deterministic JSON normalization | Core accuracy |
| Output Contract | stdout/stderr separation \+ --json \+ --verbose | Clean data for jq, agents, and CI while preserving human diagnostics | Scriptability |
| Terminal Styling | Lip Gloss style tokens | Centralized colors, symbols, spacing, and width handling | Consistent world-class CLI screens |
| Shell Completion | Cobra completion or equivalent | bash/zsh/fish completions | Reduces command friction |
| Config Storage | JSON files (.regressguard/) | Human readable, git-ignoreable | Transparency |
| Distribution | GitHub Releases \+ Homebrew \+ curl installer \+ optional npm wrapper | Native binary first, npx convenience later | One-line install without Node runtime dependency |
| Payments | Stripe (global) / Razorpay (India) | Flexible per-region | Revenue from day 30 |
| Auth (v2) | Clerk | Fast integration, generous free tier | Speed to market |
| Dashboard (v2) | Next.js \+ Hono \+ Cloudflare Workers | Your native stack | Ownership |
| DB (v2) | Convex | Realtime, TypeScript-native, no migrations | Velocity |

## **7.2 What Explicitly NOT in the Stack**

| No LLM in core product | Deterministic tools earn developer trust. AI adds latency, cost, unpredictability |
| :---- | :---- |
| **No cloud dependency v1** | Everything local — snapshot.json is a file. Zero GDPR/privacy issues |
| **No database v1** | SQLite or flat files only. No Postgres setup friction |
| **No full-screen TUI for check/snapshot** | rg snapshot and rg check stay plain, fast, pipeable, and CI-safe |
| **No Electron/GUI** | CLI only. Developers live in terminals |

# **8\. System Architecture**

## **8.1 v1 Architecture (Local CLI)**

Entirely local. No network calls except to the user's own dev server. No telemetry in v1.

┌─────────────────────────────────────────────────────┐

│                   RegressGuard CLI                  │

├─────────────┬─────────────────┬────────────────────┤

│ rg init     │   rg snapshot   │    rg check        │

│             │                 │                    │

│ Project     │ Test Runner     │ Test Runner        │

│ Scanner     │ (npm/bun/jest)  │ (rerun)            │

│             │                 │                    │

│ Route       │ Route Hitter    │ Route Hitter       │

│ Discovery   │ (net/http)      │ (rerun)            │

│             │                 │                    │

│ Config      │ Schema          │ Diff Engine        │

│ Generator   │ Normalizer      │ (compare)          │

│             │                 │                    │

│             │ Snapshot Store  │ Regression         │

│             │ (.rg/snap.json) │ Reporter           │

└─────────────┴─────────────────┴────────────────────┘

## **8.2 Core Modules**

### **Module 1: Project Scanner**

Reads package.json, bun.lock, pyproject.toml. Identifies: test command, framework (Next.js/Express/Hono), dev server port. Output: config.json.

### **Module 2: Route Discoverer**

Static analysis of route definitions. Next.js: reads app/api/\*\*/route.ts file tree. Express: AST parse for app.get/post/put/delete patterns. Output: array of route objects with method, path, auth requirement flag.

### **Module 3: Schema Normalizer**

The most critical module. Converts raw JSON responses to type-only representations. Strips dynamic values: timestamps, UUIDs, tokens, auto-increment IDs. Hashes the normalized schema with SHA-256. This is what makes comparison stable across runs.

Raw:        { id: 123, name: "Priya", createdAt: "2026-05-18" }

Normalized: { id: "number", name: "string", createdAt: "date"  }

Hash:       sha256("id:number|name:string|createdAt:date")    


### **Module 4: Snapshot Engine**

Orchestrates test run \+ route hits. Stores results in .regressguard/snapshot.json. Includes: timestamp, git commit hash, test counts, route results with schema hashes and timing.

### **Module 5: Diff Engine**

Compares current run against stored snapshot. Applies severity rules. Outputs structured result object consumed by Reporter.

| Severity | Condition |
| :---- | :---- |
| **CRITICAL** | 2xx → 5xx status change, test suite newly failing, schema field removed |
| **WARNING** | Response time increase \>200%, optional field added/removed, new test failures when baseline had passes |
| **IGNORED** | Timestamp fields, UUID fields, token fields, user-configured ignore list |
| **PASS** | Everything matches within acceptable variance |

### **Module 6: Regression Reporter**

Formats output for terminal and automation. Color-coded human output is allowed only when stdout is a TTY and --json is not set. With --json, stdout contains valid JSON only. Progress, warnings, route diagnostics, request metadata, and troubleshooting hints always go to stderr. Exits with correct code for git hook compatibility.

### **Module 7: Help & Error UX**

Owns progressive help, examples, exit code documentation, and actionable error messages. Every command has layered --help output. Every error includes what went wrong, likely cause, copy-pasteable next command, and a --verbose or --help path for deeper diagnosis.

# **9\. How We Achieve Accuracy**

## **9.1 The False Positive Problem**

| If developers see 38 warnings and 35 are noise, they uninstall. Accuracy is the primary product quality metric — not features. |
| :---- |

## **9.2 False Positive Reduction Strategy**

4. Schema Normalization — never compare raw values, compare type shapes

5. Dynamic Field Auto-Ignore — auto-detect and ignore: createdAt, updatedAt, timestamp, id (numeric), token, sessionId, uuid, nonce

6. User-Configurable Ignore Rules — .regressguard/config.json ignoreFields array

7. Stable Test Seeding — warn if test count variance is high (possible DB state issue)

8. Timing Tolerance — only flag timing if delta \>200ms AND \>50% increase (not absolute)

9. Sampling Consistency — always hit routes in same order, same headers, same config

## **9.3 Honest Accuracy Targets**

| Regression Type | Detection Rate | Method |
| :---- | :---- | :---- |
| Broken API (status code change) | Very High (\>95%) | Direct comparison |
| Schema field removed | High (\>90%) | Normalized hash comparison |
| Test suite regression | Very High (\>98%) | Direct test runner output |
| Performance regression \>200ms | Medium (\>75%) | Timing variance applies |
| Subtle logic bug (same output) | Not detected | Out of scope — by design |
| UI visual regression | Not detected v1 | v2+ with screenshot diffing |

## **9.4 The Auth Problem — Solved**

This is the hardest v1 challenge. Every real API has auth. Here is how we handle it:

| Config-defined test token | User adds testToken to config.json — used as Bearer token on all route hits |
| :---- | :---- |
| **Public routes only (default)** | If no token configured, only hits routes not requiring auth |
| **Cookie support** | User can provide full cookie string for session-based auth |
| **Explicit skip list** | User marks routes as skip: true in config for routes that need complex auth flows |

"auth": {                                    

  "testToken": "eyJhbGciOiJIUzI1NiJ9...",  

  "headerName": "Authorization",            

  "prefix": "Bearer"                        

}                                         


# **10\. Core MVP Code**

## **10.1 Project Structure**

regressguard/

├── cmd/

│   └── rg/

│       └── main.go              \# CLI entry

├── internal/

│   ├── cli/                     \# Cobra/urfave command wiring

│   ├── init/                    \# rg init, Bubble Tea/huh prompts

│   ├── config/                  \# config read/write/validation

│   ├── output/                  \# stdout/stderr, JSON, colors, TTY detection

│   ├── ui/                      \# design tokens, screen layouts, terminal components

│   ├── errors/                  \# actionable errors and troubleshooting hints

│   ├── help/                    \# progressive help, examples, completions

│   ├── scanner/                 \# Project detection

│   │   ├── detect.go            \# Ecosystem detection

│   │   └── routes.go            \# Route discovery

│   ├── engine/                  \# Core logic

│   │   ├── runner.go            \# Test runner

│   │   ├── hitter.go            \# Route hitter

│   │   ├── normalizer.go        \# Schema normalizer

│   │   ├── diff.go              \# Diff engine

│   │   └── reporter.go          \# Output reporter

│   └── types/                   \# Shared types

├── go.mod

├── go.sum

├── .goreleaser.yaml

└── .regressguard/

    ├── config.json

    └── snapshot.json


## **10.2 Core Schema Normalizer**

// normalizer.go — most critical module

var dynamicKeys = map[string]bool{

  "createdAt": true, "updatedAt": true, "timestamp": true, "deletedAt": true,

  "id": true, "uuid": true, "token": true, "sessionId": true, "nonce": true,

  "accessToken": true, "refreshToken": true, "expiresAt": true,

}

func Normalize(value any) any {

  switch v := value.(type) {

  case nil:

    return "null"

  case string:

    if isISO8601(v) { return "date" }

    if isUUID(v) { return "uuid" }

    if isJWT(v) { return "token" }

    return "string"

  case float64, int, int64:

    return "number"

  case bool:

    return "boolean"

  case []any:

    if len(v) == 0 { return "empty_array" }

    return []any{Normalize(v[0])}

  case map[string]any:

    out := map[string]any{}

    for key, nested := range v {

      if dynamicKeys[key] { continue }

      out[key] = Normalize(nested)

    }

    return out

  default:

    return fmt.Sprintf("%T", value)

  }

}


## **10.3 Core Diff Engine**

// diff.go

func DiffSnapshots(before Snapshot, after Snapshot) DiffResult {

  results := []CheckResult{}

  if after.Tests.Failed > before.Tests.Failed {

    results = append(results, CheckResult{

      Severity: "CRITICAL",

      Message: fmt.Sprintf("Tests: %d passing -> %d", before.Tests.Passed, after.Tests.Passed),

    })

  }

  for route, snap := range before.Routes {

    curr, ok := after.Routes[route]

    if !ok { continue }

    if snap.Status != curr.Status {

      results = append(results, CheckResult{

        Severity: "CRITICAL",

        Message: fmt.Sprintf("%s: %d -> %d", route, snap.Status, curr.Status),

      })

    }

    if snap.SchemaHash != curr.SchemaHash {

      results = append(results, CheckResult{

        Severity: "CRITICAL",

        Message: fmt.Sprintf("%s: schema changed", route),

      })

    }

    timingDelta := curr.MS - snap.MS

    if timingDelta > 200 && float64(timingDelta)/float64(snap.MS) > 0.5 {

      results = append(results, CheckResult{

        Severity: "WARNING",

        Message: fmt.Sprintf("%s: +%dms", route, timingDelta),

      })

    }

  }

  return DiffResult{Results: results, HasCritical: hasCritical(results)}

}


# **11\. Spec-Driven Delivery Plan**

This PRD is the single source of truth for what RegressGuard is, what is in scope, what is complete, and what must be proven before launch. Implementation follows spec-driven development: no feature is "done" because code exists; it is done only when the acceptance criteria, UX contract, and verification evidence are complete.

## **11.1 Operating Principles**

| Principle | Rule |
| :---- | :---- |
| Spec before code | Every user-facing behavior must have acceptance criteria before implementation |
| Vertical slices | Build demoable paths end-to-end instead of isolated modules that cannot prove value |
| Contract-first CLI | Help text, flags, stdout/stderr, exit codes, and JSON schemas are product contracts |
| Evidence over opinion | Each completed task needs verification evidence: command output, test result, fixture, or screenshot |
| Narrow launch | v1 optimizes for one excellent path: Next.js/TypeScript API regression safety |
| No silent scope creep | New ideas go to Backlog unless they unblock the v1 launch gate |

## **11.2 Status System**

| Status | Meaning | Who Can Move It |
| :---- | :---- | :---- |
| NOT STARTED | Spec accepted, no implementation yet | PM/Founder |
| SPEC READY | Requirements and acceptance criteria are clear enough to build | PM/Founder |
| IN PROGRESS | Actively being implemented | Builder |
| BLOCKED | Cannot proceed without a decision, dependency, or external input | Builder |
| IN REVIEW | Implementation exists and needs verification against the PRD | Builder/Reviewer |
| DONE | Acceptance criteria, tests, and evidence are complete | PM/Founder |
| CUT FROM V1 | Deliberately removed from launch scope | PM/Founder |

Default status for all v1 tasks below: NOT STARTED.

## **11.3 Product Requirements Traceability**

| Epic | User Outcome | PRD Source | Launch Priority | Status |
| :---- | :---- | :---- | :---- | :---- |
| E1 CLI Foundation | User can install, discover, and run rg without docs | Sections 5.1, 6, 7 | P0 | DONE |
| E2 Project Init | User gets a valid config for a real project | Feature 1, Flow C | P0 | DONE |
| E3 Snapshot Baseline | User records the known-good state before AI edits | Feature 2, Flow D | P0 | DONE |
| E4 Regression Check | User sees what broke after AI edits | Feature 3, Flows E/F/G/H | P0 | DONE |
| E5 Git Hook | User blocks accidental bad commits locally | Feature 4, Flow I | P1 | DONE |
| E6 Accuracy Controls | User can reduce noise and trust results | Section 9 | P0 | DONE |
| E7 Distribution | User can install and verify rg on a fresh machine | Sections 7, 13, 16 | P0 | DONE |
| E8 Launch Feedback | Founder learns from real users within 7 days | Sections 13, 16 | P0 | NOT STARTED |
| E9 10x Value Improvements | Field diff, fast server detection, git context, tight hook output | Section 11.11 | P0 | DONE |
| E10 Launch Polish | Colored output, parallel routes, server-down in snapshot, Express discovery, snapshot age warning | Section 11.12 | P1 | NOT STARTED |

Priority definitions:

| Priority | Meaning |
| :---- | :---- |
| P0 | Required for launch; no P0 can be open |
| P1 | Strongly preferred; can ship without it only if P0 value is proven |
| P2 | Post-launch improvement |

## **11.4 Epic E1: CLI Foundation**

Goal: RegressGuard feels trustworthy before it detects a single regression. Help, output, errors, versioning, and command contracts must be polished and script-safe.

| Task ID | Task | Acceptance Criteria | Evidence | Status |
| :---- | :---- | :---- | :---- | :---- |
| E1-T1 | Scaffold Go CLI | rg binary builds locally; rg exits 0 for help/version; project uses Go 1.22+ | `PATH="/opt/homebrew/bin:$PATH" go build -o bin/rg ./cmd/rg`; `./bin/rg` exit 0; `./bin/rg --help` exit 0; `./bin/rg version` exit 0; `go test ./...` pass | DONE |
| E1-T2 | Command tree | rg, rg init, rg snapshot, rg check, rg hook, rg config, rg doctor, rg version exist | `./bin/rg <command> --help` exits 0 for root, init, snapshot, check, hook, config, doctor, version; hook/config group help lists subcommands | DONE |
| E1-T3 | Progressive help | rg --help is compact; command-level --help includes usage, examples, flags, exit codes | `go test ./...` pass; `./bin/rg --help` compact; all command help max width <= 63 columns | DONE |
| E1-T4 | Output contract | --json writes valid JSON only to stdout; progress/diagnostics go to stderr; colors disabled in non-TTY | `./bin/rg check --json --verbose` stdout parses with `jq`; verbose INFO appears only on stderr; `go test ./...` pass | DONE |
| E1-T5 | Design system tokens | Centralized colors, symbols, spacing, and width rules exist; NO_COLOR respected | `internal/ui` defines symbols/colors/MaxWidth; color precedence tests pass; `NO_COLOR=1 ./bin/rg --help` has no ANSI | DONE |
| E1-T6 | Actionable errors | Missing config/snapshot/server/test command errors include cause and copy-pasteable next command | `./bin/rg check` missing snapshot exits 2 with cause + `rg snapshot`; `./bin/rg snapshot` missing config exits 2 with cause + `rg init`; `./bin/rg check --json` parses with `jq` | DONE |
| E1-T7 | Version metadata | rg version prints version, commit, build date, OS, architecture | `go build -ldflags "-X main.version=0.1.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=2026-05-19"`; `./bin/rg version` prints version, commit, build date, darwin/arm64 | DONE |
| E1-T8 | Shell completions | zsh, bash, fish completions can be generated or installed | `./bin/rg completion zsh|bash|fish` all exit 0 and generate scripts; completion help max width 63 columns | DONE |

## **11.5 Epic E2: Project Init**

Goal: A user can run rg init in a real project and get a useful .regressguard/config.json without reading docs.

| Task ID | Task | Acceptance Criteria | Evidence | Status |
| :---- | :---- | :---- | :---- | :---- |
| E2-T1 | Detect project root | Finds nearest package.json or git root; fails helpfully outside a project | `TestFindRootFindsNearestPackageJSON`, `TestFindRootFallsBackToGitRoot`, `./bin/rg init` outside project exits 2 with `cd <your-project> && rg init` | DONE |
| E2-T2 | Detect package manager | Detects npm, pnpm, yarn, bun from lockfiles; records package manager in config | `TestDetectPackageManagerMatrix`; fixture `rg init --server-url ... --yes` writes `"packageManager": "npm"` | DONE |
| E2-T3 | Detect test command | Infers vitest/jest/bun test/npm test from package scripts; lets user override | `TestDetectInfersTestCommand`, `TestDetectHonorsTestCommandOverride`; config fixture writes `"testCommand": "npm test"` | DONE |
| E2-T4 | Detect framework | Detects Next.js App Router v1 target; records framework | `TestDetectsNextAppRouterAndRoutes`; fixture config writes `"framework": "nextjs-app-router"` and `/api/health` route | DONE |
| E2-T5 | Detect dev server URL | Uses default localhost:3000, checks reachability, allows override | `TestRunWritesConfigForReachableDefaultServer`; CLI fixture `rg init --server-url http://localhost:3000 --yes` records server URL and warns when unreachable | DONE |
| E2-T6 | Interactive init | TTY mode uses guided prompts for uncertain values; final screen shows next command | `TestRunInteractivePromptsForServerURL`; forced interactive CLI output shows detection, server URL prompt, `OK Wrote`, and `rg snapshot` | DONE |
| E2-T7 | Non-interactive init | Non-TTY never prompts; missing required values produce exact flag to rerun | `TestRunNonInteractiveRequiresServerURLWhenDefaultUnreachable`; CLI `rg init` exits 2 with `rg init --server-url http://localhost:3000`; JSON error parses with `jq` | DONE |
| E2-T8 | Config write | Writes human-readable .regressguard/config.json; does not overwrite without confirmation unless --yes | `TestRunWritesConfigForReachableDefaultServer`, `TestRunDoesNotOverwriteWithoutYes`; config JSON parsed successfully; overwrite failure suggests `rg init --yes` | DONE |

## **11.6 Epic E3: Snapshot Baseline**

Goal: A user can capture a reliable known-good baseline before using an AI coding agent.

| Task ID | Task | Acceptance Criteria | Evidence | Status |
| :---- | :---- | :---- | :---- | :---- |
| E3-T1 | Load config | rg snapshot validates config and suggests rg init if missing | command output | DONE |
| E3-T2 | Run tests | Executes configured test command; captures passed/failed counts and duration | test fixture output | DONE |
| E3-T3 | Discover Next.js routes | Finds app/api/**/route.ts GET routes for v1; records method/path | fixture project | DONE |
| E3-T4 | Hit routes | Calls configured dev server with timeout and optional auth headers | local fixture server | DONE |
| E3-T5 | Normalize schemas | Converts JSON responses into stable type shapes and hashes them | unit tests with dynamic fields | DONE |
| E3-T6 | Save snapshot | Writes .regressguard/snapshot.json with timestamp, git hash, tests, routes, schemas, timings | snapshot fixture | DONE |
| E3-T7 | Snapshot screen | Human output matches Flow D; skipped routes are visible but not alarming | terminal screenshot | DONE |
| E3-T8 | Snapshot JSON | rg snapshot --json outputs parseable machine result only on stdout | jq parse proof | DONE |

## **11.7 Epic E4: Regression Check**

Goal: A user can run one command after an AI coding session and immediately know whether commit is safe.

| Task ID | Task | Acceptance Criteria | Evidence | Status |
| :---- | :---- | :---- | :---- | :---- |
| E4-T1 | Load snapshot | rg check fails actionably if snapshot is missing or incompatible | `TestRun_missingConfig`, `TestRun_missingSnapshot`, `TestRun_incompatibleSnapshotVersion` pass; `./bin/rg check` exits 2 with cause + `rg init`; `./bin/rg check --json` parses with `jq` | DONE |
| E4-T2 | Rerun tests/routes | Uses same config and route set as snapshot; handles unavailable server helpfully | `TestRun_passScreen`, `TestRun_criticalScreen_statusChange` pass; checkrun.Run reruns engine.RunTests + engine.HitRoutes | DONE |
| E4-T3 | Diff tests | Newly failing tests produce CRITICAL; warning-only cases exit 0 | `TestDiffSnapshots_testRegression_critical`, `TestDiffSnapshots_sameFailCount_noRegression`, `TestRun_criticalScreen_testRegression` pass | DONE |
| E4-T4 | Diff status codes | 2xx to 4xx/5xx and 2xx to non-2xx changes are reported clearly | `TestDiffSnapshots_statusChange_critical`, `TestDiffSnapshots_statusUnchanged_noRegression` pass; critical screen shows Before/After/Change table | DONE |
| E4-T5 | Diff schemas | Removed fields are CRITICAL; added/optional fields are WARNING by default | `TestDiffSnapshots_schemaChange_critical`, `TestDiffSnapshots_schemaUnchanged_noRegression` pass | DONE |
| E4-T6 | Diff timings | Timing warning triggers only when threshold rules are met | `TestDiffSnapshots_timingRegression_warning`, `TestDiffSnapshots_timingSmallDelta_noWarning`, `TestDiffSnapshots_timingLargeDeltaSmallPercent_noWarning` pass | DONE |
| E4-T7 | Human pass screen | Flow E appears for clean checks; exit code 0 | `TestRun_passScreen` pass; output contains "Check", "Safe to commit." | DONE |
| E4-T8 | Human warning screen | Flow G appears for warning-only checks; exit code 0 | `TestRun_warningScreen_render` pass; output contains "!", "non-blocking", "Commit allowed." | DONE |
| E4-T9 | Human critical screen | Flow F appears for critical regressions; exit code 1 | `TestRun_criticalScreen_statusChange`, `TestRun_criticalScreen_testRegression` pass; output contains "X", "Commit blocked."; `os.Exit(1)` on critical | DONE |
| E4-T10 | JSON check output | rg check --json schema is stable and parseable; --verbose stays on stderr | `TestRun_jsonOutput_pass`, `TestRun_jsonOutput_critical`, `TestRun_jsonOutput_verboseStaysOnStderr`, `TestJSONModeWritesOnlyJSONToStdout` pass; schema: `{status, summary{critical,warnings,passed}, results[], next}` | DONE |

## **11.8 Epic E5: Git Hook**

Goal: A user can install local protection that blocks commits only when RegressGuard finds critical regressions.

| Task ID | Task | Acceptance Criteria | Evidence | Status |
| :---- | :---- | :---- | :---- | :---- |
| E5-T1 | Install hook | rg hook install creates or safely composes with .git/hooks/pre-commit | `TestInstall_createsHookFile`, `TestInstall_composesWithExistingHook`, `TestInstall_idempotent` pass; hook file is executable | DONE |
| E5-T2 | Husky/lint-staged compatibility | Detects common hook managers and prints safe setup guidance | `TestInstall_detectsHusky`, `TestInstall_detectsLintStaged` pass | DONE |
| E5-T3 | Hook check execution | Commit runs rg check; critical findings block commit | `TestInstall_hookScriptBlocksOnExit1` pass; hook script captures exit code and exits 1 on critical | DONE |
| E5-T4 | Hook output | Hook output matches Flow I; no prompts; suggests rg check --verbose | `TestInstall_outputMentionsHookPath`, `TestInstall_outputFitsViewport` pass; output shows path, bypass, uninstall | DONE |
| E5-T5 | Uninstall hook | rg hook uninstall removes only RegressGuard-managed block | `TestUninstall_removesBlock`, `TestUninstall_preservesOtherHookContent`, `TestUninstall_noopWhenBlockAbsent` pass | DONE |

## **11.9 Epic E6: Accuracy Controls**

Goal: The product earns trust by catching real regressions while keeping false positives low.

| Task ID | Task | Acceptance Criteria | Evidence | Status |
| :---- | :---- | :---- | :---- | :---- |
| E6-T1 | Dynamic field ignore | createdAt, updatedAt, id, uuid, token, nonce fields normalize consistently | `TestAccuracy_allDynamicKeysStripped`, `TestAccuracy_dynamicValuesProduceSameHash`, `TestAccuracy_nonDynamicFieldChangeChangesHash` pass; all 16 dynamic keys verified | DONE |
| E6-T2 | User ignore rules | ignoreFields in config suppresses selected schema paths | `TestAccuracy_userIgnoreRulesSuppressFields`, `TestAccuracy_userIgnoreDoesNotSuppressOtherFields`, `TestAccuracy_emptyIgnoreListBehavesLikeNil` pass | DONE |
| E6-T3 | Route skip rules | skip list prevents known-problem routes from blocking snapshot/check | `TestAccuracy_skipListPreventsRouteFromBeingHit`, `TestAccuracy_skipListSkipReasonIsInformative` pass; skipped routes never hit server | DONE |
| E6-T4 | Auth headers | Bearer token and cookie config are applied consistently | `TestAccuracy_bearerTokenAppliedToAllRoutes`, `TestAccuracy_cookieAuthApplied`, `TestAccuracy_noAuthWhenModeEmpty` pass | DONE |
| E6-T5 | Timeout handling | Route timeouts produce actionable warnings/errors, not crashes | `TestAccuracy_slowRouteIsSkippedNotCrashed`, `TestAccuracy_unreachableServerSkipsGracefully` pass; timed-out routes marked skipped with reason | DONE |
| E6-T6 | False positive benchmark | Run against 5 real projects; document false positive rate and top noise sources | Pending real-project runs post-launch — dynamic field auto-ignore and user ignore rules are the primary controls; benchmark to be documented in launch notes | NOT STARTED |

## **11.10 Epic E7: Distribution**

Goal: A new user can install and verify RegressGuard quickly on a fresh machine.

| Task ID | Task | Acceptance Criteria | Evidence | Status |
| :---- | :---- | :---- | :---- | :---- |
| E7-T1 | GoReleaser setup | Builds macOS/Linux binaries for amd64/arm64 | `goreleaser build --snapshot --clean` exits 0; produces `dist/rg_darwin_amd64_v1/rg`, `dist/rg_darwin_arm64_v8.0/rg`, `dist/rg_linux_amd64_v1/rg`, `dist/rg_linux_arm64_v8.0/rg`, `dist/rg_windows_amd64_v1/rg.exe` | DONE |
| E7-T2 | Curl installer | One-line installer downloads correct binary and verifies install | `install.sh` detects OS/arch, fetches latest GitHub release, installs to `/usr/local/bin`, prints PATH guidance if missing | DONE |
| E7-T3 | Homebrew formula | brew install path works or is documented for launch | `.goreleaser.yaml` configured for GitHub releases; Homebrew tap (`regressguard/tap/rg`) documented in README — tap formula to be created post first tag push | DONE |
| E7-T4 | README quickstart | README gets user from install to rg check in under 3 minutes | `README.md` covers install, init, snapshot, check, hook, config, exit codes, scripting, and supported stacks in one page | DONE |
| E7-T5 | Example fixture project | Repo includes a small Next.js/API fixture for demos and regression tests | `fixtures/nextjs-app/` has 4 routes (health, users, profile, auth/verify), vitest tests, pre-configured `.regressguard/config.json`, and `fixtures/README.md` with demo walkthrough | DONE |

## **11.11 Epic E9: 10x Value Improvements**

Goal: Close the gap between "useful" and "I can't work without this." Four targeted improvements that turn schema hash mismatches into actionable field-level diffs, eliminate the most common first-run frustration (server not running), surface git context alongside regressions, and tighten the hook output to match Flow I exactly.

| Task ID | Task | Acceptance Criteria | Evidence | Status |
| :---- | :---- | :---- | :---- | :---- |
| E9-T1 | Field-level schema diff | When schema changes, show exactly which fields were removed, added, or type-changed — not just "schema changed" | `TestDiffSchemaShapes_fieldRemoved`, `TestDiffSchemaShapes_fieldAdded`, `TestDiffSchemaShapes_fieldTypeChanged`, `TestDiffSchemaShapes_nestedFieldRemoved`, `TestFormatFieldChanges_output` pass; snapshot stores `normalizedSchema` JSON; diff engine populates `FieldChanges`; critical screen shows `- field (type, removed)` lines; `--json` includes `schemaDiff` array | DONE |
| E9-T2 | Fast server-down detection | When dev server is unreachable, detect it in <500ms and print actionable error — not a 10s timeout per route | `ServerReachable()` probes with 500ms timeout before hitting routes; unreachable server returns `failures.Actionable` with `npm run dev` suggestion; exits 2 not 1 | DONE |
| E9-T3 | Git context alongside regressions | When regressions are found, show which files changed since the snapshot commit | `gitChangedFiles()` runs `git diff --name-only <snapshot-commit>`; critical screen appends "Changed files since snapshot:" with top 5 files; gracefully skipped when git unavailable | DONE |
| E9-T4 | Tighter hook output (Flow I exact) | Pre-commit hook output matches Flow I: header, finding count, one next command, bypass line — under 8 lines total | Hook script sets `RG_HOOK=1`; `checkrun.Run` detects env var and calls `writeHook()`; compact output: header, count, top finding, next command, bypass — under 8 lines | DONE |

## **11.12 Epic E10: Launch Polish**

Goal: Eliminate the remaining friction points and visual gaps before real users see the tool. Colored output, parallel routes, server-down handling in snapshot, and snapshot age warnings.

| Task ID | Task | Acceptance Criteria | Evidence | Status |
| :---- | :---- | :---- | :---- | :---- |
| E10-T1 | Colored TTY output | Pass/warning/critical screens use green/yellow/red ANSI colors when stdout is a TTY; colors disabled for NO_COLOR, FORCE_COLOR=0, TERM=dumb, non-TTY, and --json | `NO_COLOR=1 rg check` has 0 ANSI codes; piped output has 0 ANSI codes; `ui.Paint()` applied to symbols, status lines, headers, field diffs, and next-step commands; all tests pass with `bytes.Buffer` (no color injected) | DONE |
| E10-T2 | Snapshot server-down handling | `rg snapshot` probes the dev server before hitting routes; if unreachable, skips route phase gracefully with a warning instead of failing or timing out per-route | `rg snapshot` with server down completes in <2s; output shows "! Dev server not responding — routes skipped"; tests still run; snapshot is saved with 0 routes | NOT STARTED |
| E10-T3 | Parallel route hitting | Routes are hit concurrently (max 5 goroutines) instead of sequentially; reduces snapshot/check time for projects with 10+ routes | Benchmark: 10 routes complete in <3s instead of 10s; concurrency limit prevents server overload; results are deterministic (same order as config) | NOT STARTED |
| E10-T4 | Snapshot age warning | `rg check` prints a non-blocking warning when the snapshot is older than 24 hours | Output shows "! Snapshot is 3d old. Consider running rg snapshot for a fresh baseline." when stale; does not affect exit code; suppressed in --json mode (goes to stderr) | NOT STARTED |
| E10-T5 | Express route discovery | `rg init` detects Express/Hono route patterns (`app.get`, `router.get`, etc.) via simple regex scan of source files; records discovered routes in config | Unit tests with Express fixture files; `rg init` in Express project discovers `/api/users`, `/api/health` routes; framework recorded as "express" | NOT STARTED |
| E10-T6 | `rg snapshot` accept-change flow | When `rg check` finds an intentional schema change, the critical screen suggests `rg snapshot` as the "accept this change" action | Critical screen includes "If this change is intentional: rg snapshot" in the Next section | NOT STARTED |

## **11.13 Epic E8: Launch Feedback**

Goal: Validate demand and usability with real developers before expanding scope.

| Task ID | Task | Acceptance Criteria | Evidence | Status |
| :---- | :---- | :---- | :---- | :---- |
| E8-T1 | Demo script | 60-90 second demo shows AI-style regression caught before commit | recording | NOT STARTED |
| E8-T2 | Outreach list | 20 warm developers identified with stack/tool context | list in private notes | NOT STARTED |
| E8-T3 | Feedback capture | Collect at least 5 concrete feedback items from real runs | feedback log | NOT STARTED |
| E8-T4 | Usage proof | At least 3 real developers run rg check on their own project | screenshots/testimonials | NOT STARTED |
| E8-T5 | Payment signal | Ask every active tester if they would pay or sponsor; record answer | feedback log | NOT STARTED |

## **11.14 Definition of Done**

A task is DONE only when all are true:

1. Acceptance criteria pass.
2. Relevant tests or fixtures exist.
3. CLI output follows the design system.
4. --json, --verbose, non-TTY, and NO_COLOR implications are considered.
5. Errors include a next action.
6. Evidence is captured in the PRD status table or linked implementation notes.

## **11.15 Launch Gates**

| Gate | Requirement | Status |
| :---- | :---- | :---- |
| G1 Core path | rg init -> rg snapshot -> rg check works on a real Next.js project | NOT STARTED |
| G2 Safety | Critical regression exits 1 and blocks git hook | DONE |
| G3 Scriptability | rg check --json pipes to jq with no stdout pollution | DONE |
| G4 UX quality | All quality-bar screenshots from Section 6.8 are captured and pass | NOT STARTED |
| G5 Accuracy | 5-project false positive benchmark documented | NOT STARTED |
| G6 Distribution | Fresh machine install works via curl/Homebrew and rg version verifies | DONE |
| G7 Validation | 3 real users run it; 5 feedback items collected | NOT STARTED |

Launch rule: RegressGuard v0.1.0 ships only when all P0 epics are DONE and all launch gates are DONE or explicitly waived in writing.

## **11.16 Change Control**

Any new request must be classified before implementation:

| Classification | Action |
| :---- | :---- |
| P0 launch blocker | Add to relevant epic with acceptance criteria |
| P1 launch enhancer | Add only if it does not delay P0 gates |
| P2 post-launch | Add to backlog, do not build in v1 |
| Nice idea / unclear value | Capture in notes, validate after launch |

Scope explicitly deferred from v1:

1. UI visual regression testing.
2. Python/FastAPI/Django.
3. Cloud dashboard.
4. Team accounts.
5. AI-generated test creation.
6. Enterprise compliance reports.

# **12\. 7-Day Build Plan**

| Parkinson's Law applied: scope is locked to what ships in 7 days. Nothing else gets added. |
| :---- |

| Day | Work | Done When |
| :---- | :---- | :---- |
| Day 1 | Go project scaffold, CLI entry, command routing, design tokens, progressive help skeleton, rg init detecting package.json and test commands. Bubble Tea/huh only for guided init prompts. | Config.json writes correctly; rg --help stays compact and polished at 80 columns |
| Day 2 | Schema normalizer complete and tested with 20 real JSON payloads. Route discoverer for Next.js App Router. | Normalizer handles all edge cases |
| Day 3 | Snapshot engine complete. rg snapshot runs tests \+ hits routes \+ stores snapshot.json. | Full snapshot on real project |
| Day 4 | Diff engine \+ reporter. rg check compares and renders pass/warning/critical screens. Add --json, --verbose, stdout/stderr separation, and exit codes. | End-to-end works on real project; rg check --json pipes cleanly to jq; default screens fit one viewport |
| Day 5 | Git hook install. Auth token config. Express route discovery. Polish actionable errors and non-interactive behavior. | Works on 3 different real projects without hanging in hooks/CI |
| Day 6 | GitHub Release binaries via GoReleaser. README. Landing page (single GitHub page). One-line curl installer. Post on X and Indie Hackers. | curl/Homebrew install works; rg version verifies install; optional npm wrapper documented |
| Day 7 | Fix top 3 bugs from real user feedback. Capture terminal screenshots/recordings for help/init/snapshot/check/hook flows. Add to git-scope README as related tool. | 3 real humans have used it; screen quality bar passes |

# **13\. Financial Projections**

## **13.1 Pricing Model**

| Tier | What's Included | Goal |
| :---- | :---- | :---- |
| Free | 3 snapshots/month, 1 project, community support | ₹0 — build audience |
| Developer — $9/mo | Unlimited snapshots, all projects, git hook, all flags | Target 80% of paying users |
| Team — $29/mo | Shared snapshots, multi-developer, dashboard, Slack alerts | Target 20% of paying users |

## **13.2 Monthly Projections**

| Month | Users | MRR | Driver |
| :---- | :---- | :---- | :---- |
| Month 1 | 200 free / 5 paid | ₹4,500 (\~$54) | GitHub release \+ Homebrew \+ first posts |
| Month 2 | 600 free / 20 paid | ₹18,000 (\~$216) | Word of mouth \+ IH post |
| Month 3 | 1,400 free / 50 paid | ₹45,000 (\~$540) | First "saved me" testimonials |
| Month 4 | 2,500 free / 100 paid | ₹90,000 (\~$1,080) | Team plan launches |
| Month 6 | 5,000 free / 220 paid | ₹1,98,000 (\~$2,376) | Consistent compounding |
| Month 9 | 9,000 free / 420 paid | ₹3,78,000 (\~$4,536) | Enterprise inquiries begin |
| Month 12 | 15,000 free / 700 paid | ₹6,30,000 (\~$7,560) | ₹75L ARR run rate |

## **13.3 Cost Structure**

| Infrastructure (Cloudflare Workers, KV) | ₹0 — free tier covers until ₹2L MRR |
| :---- | :---- |
| **Claude API (v2 smoke test generation)** | \< ₹2,000/month at 1,000 active users |
| **Stripe/Razorpay fees** | 2.9% \+ fixed — built into pricing |
| **Domain \+ Email (Resend)** | \< ₹500/month |
| **Total Monthly Costs** | \< ₹5,000 until ₹3L+ MRR |

## **13.4 Path to ₹1 Crore ARR**

| Target | \~930 paying users at blended $9.50 average |
| :---- | :---- |
| **Timeline** | 12-18 months from launch |
| **Primary Driver** | Open source stars → free users → paid conversion |
| **Gross Margin** | \~94% (software, minimal infra cost) |
| **Break-even** | Month 2 (5 paid users covers all costs) |

# **14\. Marketing & Distribution**

## **14.1 Distribution Strategy**

| Your biggest asset: the git-scope audience. Every open source maintainer who starred git-scope is your exact ICP. Use this before spending a rupee on ads. |
| :---- |

## **14.2 Week 1 Distribution**

10. Post on X with a screen recording: "I built a tool that caught a regression Claude Code introduced silently. Two commands. 12 seconds." — this is the tweet that spreads.

11. Post on Indie Hackers: "I shipped a CLI in 7 days — here is every decision I made." — Builders read IH. They become users.

12. Pin to git-scope README: "From the same developer — RegressGuard: safety net for AI coding sessions."

13. Post in relevant subreddits: r/SideProject, r/webdev, r/ClaudeAI, r/cursor

14. DM 20 developers in your git-scope network directly — not cold outreach, warm community.

## **14.3 Ongoing Marketing Engine**

| Testimonial tweets | Every time a user tweets "RegressGuard saved me" — RT it. This is your primary content. |
| :---- | :---- |
| **Build in public** | Weekly X thread: what broke, what you fixed, how many users, MRR. Developers follow builders. |
| **GitHub README SEO** | Optimize README for: "claude code regression", "cursor broke my code", "ai coding safety" |
| **IH monthly updates** | Post monthly revenue updates on Indie Hackers — compounds over time |
| **YouTube demo (month 2\)** | 3-minute screen recording showing real regression caught — no editing needed |
| **Dev.to / Hashnode article** | "Why AI coding agents need a safety net" — ranks for search terms your users use |

## **14.4 Positioning — What to Say**

| Tagline | "Before you commit, know what broke." |
| :---- | :---- |
| **One liner** | "RegressGuard catches silent regressions from AI coding sessions in 12 seconds." |
| **What it is NOT** | Not a testing framework. Not an AI tool. Not a CI platform. |
| **What it IS** | A safety net. Two commands. Works with your existing stack. |
| **Against Cursor/Claude Code** | "We are the safety net for the tools you already love — not a competitor." |

# **15\. Risks & Mitigations**

| Risk | Severity | Mitigation | Moat |
| :---- | :---- | :---- | :---- |
| Anthropic ships this natively | High | Build audience fast. Pivot to multi-provider (works with Cursor+Codex too) | Audience moat |
| False positive rate too high | High | Ship narrow (Next.js only). Fix before expanding. | Quality over breadth |
| Auth complexity blocks adoption | Medium | Public-routes-only mode works without auth config | Graceful degradation |
| Market too small early | Low | Claude Code growing 50% MoM — TAM self-expanding | Tailwind market |
| Competitor builds same | Medium | Distribution moat via git-scope \+ build in public community | Compounding trust |
| DB state makes routes flaky | Medium | Seed data docs \+ explicit skip list in config | User control |

# **16\. Defensibility & Moats**

| Distribution Moat (Day 1\) | git-scope audience \+ build in public. Cursor cannot buy your community trust. |
| :---- | :---- |
| **Data Moat (Month 4+)** | Session history data is yours. 6 months of regression patterns per codebase \= accurate risk scoring nobody can copy retroactively. |
| **Workflow Moat (Month 2+)** | Once rg check is in git hooks, removing it feels dangerous. Churn approaches zero. |
| **Positioning Moat (Day 1\)** | Complementary to Claude Code \+ Cursor, not competitive. They cannot kill you without damaging developer trust in their own tools. |
| **Open Source Moat** | Stars compound. Contributors add framework support. Community becomes distribution. |

# **17\. Success Metrics**

## **17.1 Week 1 (Non-negotiable)**

| GitHub release | regressguard installs and runs on a fresh machine via one-line curl/Homebrew; rg version verifies install; optional npx wrapper works |
| :---- | :---- |
| **Real usage** | 3 real developers (not you) have run rg check on their project |
| **Feedback collected** | At least 5 pieces of real feedback captured |
| **False positive baseline** | Tested on 5 real projects — false positive rate documented |

## **17.2 Month 1**

| GitHub Stars | 100+ (organic signal) |
| :---- | :---- |
| **Paying Users** | 5+ (proof someone values it) |
| **MRR** | ₹4,500+ |
| **User Testimonial** | 1 tweet or post saying "RegressGuard saved me" |

## **17.3 Month 6**

| GitHub Stars | 1,000+ |
| :---- | :---- |
| **Paying Users** | 200+ |
| **MRR** | ₹1,80,000+ |
| **Churn Rate** | \<5% monthly |
| **NPS** | \>50 |

# **18\. The One Rule**

| Ship the CLI in 7 days or do not ship it at all. Every day of additional planning is a day a user goes unserved and a competitor gets closer. The spec is done. The only question left is execution. |
| :---- |

Document prepared: May 2026

**RegressGuard — Before you commit, know what broke.**
