**PRODUCT REQUIREMENTS DOCUMENT**

**RegressGuard**

The Safety Net for AI-Generated Code

| Version | v1.0 — MVP |
| :---- | :---- |
| **Author** | Bharath |
| **Date** | May 2026 |
| **Status** | Ready to Build |
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
| **Stack** | Node.js / Bun / TypeScript — Next.js, Express, Hono, Vitest, Jest |
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

### **Feature 4: Git Hook Integration**

| Command | rg hook install |
| :---- | :---- |
| **Purpose** | Auto-run rg check on every git commit attempt |
| **AC 1** | Installs pre-commit hook in .git/hooks/ |
| **AC 2** | Blocks commit if CRITICAL regressions found |
| **AC 3** | Allows \--no-verify bypass for emergencies |
| **AC 4** | Works with husky and lint-staged setups |

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

# **6\. Technology Stack**

## **6.1 Stack Decisions**

| Layer | Choice | Reason | Why This |
| :---- | :---- | :---- | :---- |
| Runtime | Bun 1.x | Fast installs, built-in test runner, TypeScript native | Performance \+ your default |
| CLI Framework | @clack/prompts | Beautiful interactive prompts, zero config | DX quality |
| Language | TypeScript (strict) | Type safety across the entire CLI | Reliability |
| HTTP Testing | Built-in fetch (Bun) | No extra deps for route hitting | Minimal footprint |
| Schema Hashing | object-hash \+ custom normalizer | Deterministic JSON normalization | Core accuracy |
| Config Storage | JSON files (.regressguard/) | Human readable, git-ignoreable | Transparency |
| Distribution | npm package \+ npx | Works day one, no install required | Zero friction |
| Payments | Stripe (global) / Razorpay (India) | Flexible per-region | Revenue from day 30 |
| Auth (v2) | Clerk | Fast integration, generous free tier | Speed to market |
| Dashboard (v2) | Next.js \+ Hono \+ Cloudflare Workers | Your native stack | Ownership |
| DB (v2) | Convex | Realtime, TypeScript-native, no migrations | Velocity |

## **6.2 What Explicitly NOT in the Stack**

| No LLM in core product | Deterministic tools earn developer trust. AI adds latency, cost, unpredictability |
| :---- | :---- |
| **No cloud dependency v1** | Everything local — snapshot.json is a file. Zero GDPR/privacy issues |
| **No database v1** | SQLite or flat files only. No Postgres setup friction |
| **No Electron/GUI** | CLI only. Developers live in terminals |

# **7\. System Architecture**

## **7.1 v1 Architecture (Local CLI)**

Entirely local. No network calls except to the user's own dev server. No telemetry in v1.

┌─────────────────────────────────────────────────────┐

│                   RegressGuard CLI                  │

├─────────────┬─────────────────┬────────────────────┤

│ rg init     │   rg snapshot   │    rg check        │

│             │                 │                    │

│ Project     │ Test Runner     │ Test Runner        │

│ Scanner     │ (bun test/jest) │ (rerun)            │

│             │                 │                    │

│ Route       │ Route Hitter    │ Route Hitter       │

│ Discovery   │ (fetch)         │ (rerun)            │

│             │                 │                    │

│ Config      │ Schema          │ Diff Engine        │

│ Generator   │ Normalizer      │ (compare)          │

│             │                 │                    │

│             │ Snapshot Store  │ Regression         │

│             │ (.rg/snap.json) │ Reporter           │

└─────────────┴─────────────────┴────────────────────┘

## **7.2 Core Modules**

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

Formats output for terminal. Color-coded. Human-readable. Exits with correct code for git hook compatibility. Optionally outputs JSON for CI pipeline consumption.

# **8\. How We Achieve Accuracy**

## **8.1 The False Positive Problem**

| If developers see 38 warnings and 35 are noise, they uninstall. Accuracy is the primary product quality metric — not features. |
| :---- |

## **8.2 False Positive Reduction Strategy**

4. Schema Normalization — never compare raw values, compare type shapes

5. Dynamic Field Auto-Ignore — auto-detect and ignore: createdAt, updatedAt, timestamp, id (numeric), token, sessionId, uuid, nonce

6. User-Configurable Ignore Rules — .regressguard/config.json ignoreFields array

7. Stable Test Seeding — warn if test count variance is high (possible DB state issue)

8. Timing Tolerance — only flag timing if delta \>200ms AND \>50% increase (not absolute)

9. Sampling Consistency — always hit routes in same order, same headers, same config

## **8.3 Honest Accuracy Targets**

| Regression Type | Detection Rate | Method |
| :---- | :---- | :---- |
| Broken API (status code change) | Very High (\>95%) | Direct comparison |
| Schema field removed | High (\>90%) | Normalized hash comparison |
| Test suite regression | Very High (\>98%) | Direct test runner output |
| Performance regression \>200ms | Medium (\>75%) | Timing variance applies |
| Subtle logic bug (same output) | Not detected | Out of scope — by design |
| UI visual regression | Not detected v1 | v2+ with screenshot diffing |

## **8.4 The Auth Problem — Solved**

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


# **9\. Core MVP Code**

## **9.1 Project Structure**

regressguard/                    

├── src/                         

│   ├── cli.ts          \# Entry  

│   ├── init.ts         \# rg init

│   ├── snapshot.ts     \# rg snapshot

│   ├── check.ts        \# rg check

│   ├── scanner/        \# Project detection

│   │   ├── detect.ts   \# Ecosystem detection

│   │   └── routes.ts   \# Route discovery

│   ├── engine/         \# Core logic

│   │   ├── runner.ts   \# Test runner

│   │   ├── hitter.ts   \# Route hitter

│   │   ├── normalizer.ts \# Schema normalizer

│   │   ├── diff.ts     \# Diff engine

│   │   └── reporter.ts \# Output reporter

│   └── types.ts        \# Shared types

├── package.json                 

└── .regressguard/               

    ├── config.json              

    └── snapshot.json          


## **9.2 Core Schema Normalizer**

// normalizer.ts — most critical module          

const DYNAMIC\_KEYS \= new Set(\[                   

  "createdAt","updatedAt","timestamp","deletedAt"

  "id","uuid","token","sessionId","nonce",        

  "accessToken","refreshToken","expiresAt"        

\]);                                              

                                                 

export function normalize(obj: unknown): unknown {

  if (obj \=== null) return "null";               

  if (typeof obj \=== "string") {                 

    if (isISO8601(obj)) return "date";           

    if (isUUID(obj)) return "uuid";              

    if (isJWT(obj)) return "token";              

    return "string";                             

  }                                             

  if (typeof obj \=== "number") return "number";  

  if (typeof obj \=== "boolean") return "boolean";

  if (Array.isArray(obj)) {                     

    return obj.length \> 0 ? \[normalize(obj\[0\])\] 

      : "empty\_array";                          

  }                                             

  if (typeof obj \=== "object") {                

    return Object.fromEntries(                   

      Object.entries(obj)                       

        .filter((\[k\]) \=\> \!DYNAMIC\_KEYS.has(k))  

        .map((\[k, v\]) \=\> \[k, normalize(v)\])      

    );                                          

  }                                             

  return typeof obj;                            

}                                             


## **9.3 Core Diff Engine**

// diff.ts                                         

export function diffSnapshots(                     

  before: Snapshot,                               

  after: Snapshot                                 

): DiffResult {                                   

  const results: CheckResult\[\] \= \[\];              

                                                   

  // 1\. Test suite diff                           

  if (after.tests.failed \> before.tests.failed) { 

    results.push({ severity: "CRITICAL",          

      message: "Tests: " \+ before.tests.passed \+  

        " passing \-\> " \+ after.tests.passed });   

  }                                               

                                                   

  // 2\. Route diffs                               

  for (const \[route, snap\] of Object.entries(     

    before.routes)) {                             

    const curr \= after.routes\[route\];             

    if (\!curr) continue;                          

    // Status code regression                     

    if (snap.status \!== curr.status) {            

      results.push({ severity: "CRITICAL",        

        message: \`${route}: ${snap.status}→       

          ${curr.status}\` });                     

    }                                             

    // Schema regression                          

    if (snap.schemaHash \!== curr.schemaHash) {    

      results.push({ severity: "CRITICAL",        

        message: \`${route}: schema changed\` });   

    }                                             

    // Performance regression                     

    const timingDelta \= curr.ms \- snap.ms;        

    if (timingDelta \> 200 &&                      

      timingDelta / snap.ms \> 0.5) {             

      results.push({ severity: "WARNING",         

        message: \`${route}: \+${timingDelta}ms\` });

    }                                             

  }                                               

  return { results,                               

    hasCritical: results.some(                    

      r \=\> r.severity \=== "CRITICAL") };          

}                                               


# **10\. 7-Day Build Plan**

| Parkinson's Law applied: scope is locked to what ships in 7 days. Nothing else gets added. |
| :---- |

| Day | Work | Done When |
| :---- | :---- | :---- |
| Day 1 | Project scaffold, CLI entry, @clack/prompts working, rg init detecting package.json and test commands. Hardcoded output to verify format. | Config.json writes correctly |
| Day 2 | Schema normalizer complete and tested with 20 real JSON payloads. Route discoverer for Next.js App Router. | Normalizer handles all edge cases |
| Day 3 | Snapshot engine complete. rg snapshot runs tests \+ hits routes \+ stores snapshot.json. | Full snapshot on real project |
| Day 4 | Diff engine \+ reporter. rg check compares and outputs colored terminal report. Exit code 1 on CRITICAL. | End-to-end works on real project |
| Day 5 | Git hook install. Auth token config. Express route discovery. Polish error messages. | Works on 3 different real projects |
| Day 6 | npm publish. README. Landing page (single GitHub page). Post on X and Indie Hackers. | npx regressguard init works globally |
| Day 7 | Fix top 3 bugs from real user feedback. Add to git-scope README as related tool. | 3 real humans have used it |

# **11\. Financial Projections**

## **11.1 Pricing Model**

| Tier | What's Included | Goal |
| :---- | :---- | :---- |
| Free | 3 snapshots/month, 1 project, community support | ₹0 — build audience |
| Developer — $9/mo | Unlimited snapshots, all projects, git hook, all flags | Target 80% of paying users |
| Team — $29/mo | Shared snapshots, multi-developer, dashboard, Slack alerts | Target 20% of paying users |

## **11.2 Monthly Projections**

| Month | Users | MRR | Driver |
| :---- | :---- | :---- | :---- |
| Month 1 | 200 free / 5 paid | ₹4,500 (\~$54) | npm publish \+ first posts |
| Month 2 | 600 free / 20 paid | ₹18,000 (\~$216) | Word of mouth \+ IH post |
| Month 3 | 1,400 free / 50 paid | ₹45,000 (\~$540) | First "saved me" testimonials |
| Month 4 | 2,500 free / 100 paid | ₹90,000 (\~$1,080) | Team plan launches |
| Month 6 | 5,000 free / 220 paid | ₹1,98,000 (\~$2,376) | Consistent compounding |
| Month 9 | 9,000 free / 420 paid | ₹3,78,000 (\~$4,536) | Enterprise inquiries begin |
| Month 12 | 15,000 free / 700 paid | ₹6,30,000 (\~$7,560) | ₹75L ARR run rate |

## **11.3 Cost Structure**

| Infrastructure (Cloudflare Workers, KV) | ₹0 — free tier covers until ₹2L MRR |
| :---- | :---- |
| **Claude API (v2 smoke test generation)** | \< ₹2,000/month at 1,000 active users |
| **Stripe/Razorpay fees** | 2.9% \+ fixed — built into pricing |
| **Domain \+ Email (Resend)** | \< ₹500/month |
| **Total Monthly Costs** | \< ₹5,000 until ₹3L+ MRR |

## **11.4 Path to ₹1 Crore ARR**

| Target | \~930 paying users at blended $9.50 average |
| :---- | :---- |
| **Timeline** | 12-18 months from launch |
| **Primary Driver** | Open source stars → free users → paid conversion |
| **Gross Margin** | \~94% (software, minimal infra cost) |
| **Break-even** | Month 2 (5 paid users covers all costs) |

# **12\. Marketing & Distribution**

## **12.1 Distribution Strategy**

| Your biggest asset: the git-scope audience. Every open source maintainer who starred git-scope is your exact ICP. Use this before spending a rupee on ads. |
| :---- |

## **12.2 Week 1 Distribution**

10. Post on X with a screen recording: "I built a tool that caught a regression Claude Code introduced silently. Two commands. 12 seconds." — this is the tweet that spreads.

11. Post on Indie Hackers: "I shipped a CLI in 7 days — here is every decision I made." — Builders read IH. They become users.

12. Pin to git-scope README: "From the same developer — RegressGuard: safety net for AI coding sessions."

13. Post in relevant subreddits: r/SideProject, r/webdev, r/ClaudeAI, r/cursor

14. DM 20 developers in your git-scope network directly — not cold outreach, warm community.

## **12.3 Ongoing Marketing Engine**

| Testimonial tweets | Every time a user tweets "RegressGuard saved me" — RT it. This is your primary content. |
| :---- | :---- |
| **Build in public** | Weekly X thread: what broke, what you fixed, how many users, MRR. Developers follow builders. |
| **GitHub README SEO** | Optimize README for: "claude code regression", "cursor broke my code", "ai coding safety" |
| **IH monthly updates** | Post monthly revenue updates on Indie Hackers — compounds over time |
| **YouTube demo (month 2\)** | 3-minute screen recording showing real regression caught — no editing needed |
| **Dev.to / Hashnode article** | "Why AI coding agents need a safety net" — ranks for search terms your users use |

## **12.4 Positioning — What to Say**

| Tagline | "Before you commit, know what broke." |
| :---- | :---- |
| **One liner** | "RegressGuard catches silent regressions from AI coding sessions in 12 seconds." |
| **What it is NOT** | Not a testing framework. Not an AI tool. Not a CI platform. |
| **What it IS** | A safety net. Two commands. Works with your existing stack. |
| **Against Cursor/Claude Code** | "We are the safety net for the tools you already love — not a competitor." |

# **13\. Risks & Mitigations**

| Risk | Severity | Mitigation | Moat |
| :---- | :---- | :---- | :---- |
| Anthropic ships this natively | High | Build audience fast. Pivot to multi-provider (works with Cursor+Codex too) | Audience moat |
| False positive rate too high | High | Ship narrow (Next.js only). Fix before expanding. | Quality over breadth |
| Auth complexity blocks adoption | Medium | Public-routes-only mode works without auth config | Graceful degradation |
| Market too small early | Low | Claude Code growing 50% MoM — TAM self-expanding | Tailwind market |
| Competitor builds same | Medium | Distribution moat via git-scope \+ build in public community | Compounding trust |
| DB state makes routes flaky | Medium | Seed data docs \+ explicit skip list in config | User control |

# **14\. Defensibility & Moats**

| Distribution Moat (Day 1\) | git-scope audience \+ build in public. Cursor cannot buy your community trust. |
| :---- | :---- |
| **Data Moat (Month 4+)** | Session history data is yours. 6 months of regression patterns per codebase \= accurate risk scoring nobody can copy retroactively. |
| **Workflow Moat (Month 2+)** | Once rg check is in git hooks, removing it feels dangerous. Churn approaches zero. |
| **Positioning Moat (Day 1\)** | Complementary to Claude Code \+ Cursor, not competitive. They cannot kill you without damaging developer trust in their own tools. |
| **Open Source Moat** | Stars compound. Contributors add framework support. Community becomes distribution. |

# **15\. Success Metrics**

## **15.1 Week 1 (Non-negotiable)**

| npm publish | npx regressguard init works on a fresh machine |
| :---- | :---- |
| **Real usage** | 3 real developers (not you) have run rg check on their project |
| **Feedback collected** | At least 5 pieces of real feedback captured |
| **False positive baseline** | Tested on 5 real projects — false positive rate documented |

## **15.2 Month 1**

| GitHub Stars | 100+ (organic signal) |
| :---- | :---- |
| **Paying Users** | 5+ (proof someone values it) |
| **MRR** | ₹4,500+ |
| **User Testimonial** | 1 tweet or post saying "RegressGuard saved me" |

## **15.3 Month 6**

| GitHub Stars | 1,000+ |
| :---- | :---- |
| **Paying Users** | 200+ |
| **MRR** | ₹1,80,000+ |
| **Churn Rate** | \<5% monthly |
| **NPS** | \>50 |

# **16\. The One Rule**

| Ship the CLI in 7 days or do not ship it at all. Every day of additional planning is a day a user goes unserved and a competitor gets closer. The spec is done. The only question left is execution. |
| :---- |

Document prepared: May 2026

**RegressGuard — Before you commit, know what broke.**