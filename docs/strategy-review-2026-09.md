# RegressGuard — Strategy Review (2026-09-24, day ~69 of the 90-day gate)

_Lenses: PM · CEO · CFO · CMO · CTO · market research · continue/pivot. Every claim is
tagged **[measured]** (pulled from GitHub/HN/registry APIs today), **[sourced]** (external,
linked), or **[assumption]** (stated so it can be tested)._

## TL;DR

1. **The gate didn't fail. It was never run.** Show HN was never posted and the launch
   week-log is empty. Only 1 of the 4 launch channels (the MCP registry) went live, and
   there have been no commits for 68 days. Zero stars is a statement about distribution.
   It isn't yet a statement about demand.
2. **The main install path is broken for most developers.** The Claude Code plugin's
   `.mcp.json` runs bare `rg`, which resolves to ripgrep on any machine that has it.
   Verified on the maintainer's machine today: the `regressguard` MCP server fails with
   "Connection closed".
3. **The market is real and paying.** Money is flowing into AI-code verification
   (CodeRabbit $50M ARR, Cursor bought Graphite, Anthropic charges $15–25 per PR review).
   The risk is not "no market". The risk is that "run tests before done" becomes a
   15-line Stop hook that every harness ships.
4. **Recommendation: don't pivot on zero data.** Spend 5 days fixing the install path,
   then run a real 30-day gate (Oct 1–31) with the positioning sharpened to
   *"the contract guard that runs as your agent's Stop hook"*. Pivot criteria and the
   pivot candidate are defined in §8. Pre-validate the pivot with a landing page before
   writing any code for it.

---

## 1. Ground truth: traction data [measured, 2026-09-24]

| Metric | Value | Gate target | Note |
|---|---|---|---|
| GitHub stars | **1** | ~200 | 0.5% of target |
| Repo page views, last 14d | **0** | — | Nobody is visiting the repo |
| Clones, last 14d | 47 (26 unique) | — | With 0 views, likely bots/mirrors [assumption] |
| Binary downloads (all releases) | **2** | — | Both darwin/arm64; plausibly the maintainer |
| `.mcpb` bundle downloads | 172 | — | With 0 views, likely registry crawlers/indexers [assumption] |
| Repos with weekly `rg check` | **unmeasurable** | ≥25 | There is no telemetry, so the gate can't be scored |
| Show HN posts (Algolia) | **0** | 1 | Never posted |
| Last commit | 2026-07-18 | — | 68 days idle |
| MCP registry listing | v0.1.0 | — | Stale; latest release is v0.1.1 |

**Reading:** this is a "never shipped to humans" result. The only live channel is the
official MCP registry, a passive shelf with 12,500+ servers where fewer than 5% earn
anything ([MCP Marketplace](https://mcp-marketplace.io/blog/state-of-mcp-monetization-2026),
[digitalapplied](https://www.digitalapplied.com/blog/mcp-adoption-statistics-2026-model-context-protocol)).
Listing there without active promotion predictably yields ~0 humans.

## 2. Market research: is the problem worth money? [sourced]

**The pain is measurable and growing**
- 66% of developers say their top frustration is AI output that is "almost right, but
  not quite". Trust in AI accuracy fell to 29%, and 46% actively distrust it
  ([Stack Overflow 2025](https://survey.stackoverflow.co/2025/ai)).
- DORA 2025: 90% of respondents use AI. AI adoption now correlates positively with
  throughput but **still negatively with delivery stability**. Instability rises where
  teams adopt AI "without rethinking quality gates"
  ([Google Cloud / DORA](https://cloud.google.com/blog/products/ai-machine-learning/announcing-the-2025-dora-report),
  [RedMonk](https://redmonk.com/rstephens/2025/12/18/dora2025/)).
  That is RegressGuard's thesis, stated by Google.
- Volume: AI agents opened 17M PRs on GitHub in March 2026, 4× in six months. Claude Code
  alone makes ~2.6M commits/week
  ([danilchenko](https://www.danilchenko.dev/posts/2026-04-11-github-ai-agents-pull-requests/),
  [quasa](https://quasa.io/media/github-s-ai-agent-tsunami-275-million-commits-a-week-14-billion-projected-for-2026-and-the-platform-is-starting-to-crack)).
- Vibe-coded apps break in production:
  - A Lovable RLS flaw (CVE-2025-48757) exposed 170+ apps.
  - A scan of ~380k vibe-coded apps found ~5,000 leaking data.
  - 63% of 62 audited Lovable apps had critical or high findings.
  - Sources: [Superblocks](https://www.superblocks.com/blog/lovable-vulnerabilities),
    [vibegraveyard](https://vibegraveyard.ai/story/redaccess-vibe-coded-apps-380k-data-exposure-study/),
    [TNW](https://thenextweb.com/news/lovable-vibe-coding-security-crisis-exposed).

**Money is flowing into verification**
- CodeRabbit: $25M → $50M ARR (Dec 2025 → Jul 2026), $143M Series C at a $1.5B valuation,
  17k customers
  ([TFN](https://techfundingnews.com/coderabbit-lands-143m-at-1-5b-valuation-as-ai-generated-code-surges/),
  [Sacra](https://sacra.com/c/coderabbit/)).
- The pure-play AI code review segment grew from ~$180M to ~$420M ARR (2025 → 2026)
  ([Sacra](https://sacra.com/c/coderabbit/)).
- Cursor acquired Graphite for "way over" its $290M valuation
  ([TechCrunch](https://techcrunch.com/2025/12/19/cursor-continues-acquisition-spree-with-graphite-deal)).
- Anthropic Code Review costs **$15–25 per PR** (Team/Enterprise only)
  ([Claude docs](https://code.claude.com/docs/en/code-review),
  [dev.to](https://dev.to/umesh_malik/anthropic-code-review-for-claude-code-multi-agent-pr-reviews-pricing-setup-and-limits-3o35)).
  That price is a strong willingness-to-pay anchor for "verify this change".
- Momentic raised a $15M Series A and TestSprite a $6.7M seed, both for testing in
  AI-native development
  ([TechCrunch](https://techcrunch.com/2025/11/24/momentic-raises-15m-to-automate-software-testing/),
  [PRNewswire](https://www.prnewswire.com/news-releases/testsprite-raises-6-7-million-seed-round-to-become-the-testing-backbone-of-the-ai-native-development-era-302598377.html)).
- Keploy is still at a $1.3M seed from 2022
  ([Tracxn](https://tracxn.com/d/companies/keploy/__lTKMBFH3EscRafIRVxo_LNmjVc2M3TkfO72VRNMrhrs)).
  Deep record/replay tech alone has not attracted capital.
- The AI-enabled testing market is ~$1.0B (2025), projected at ~$4.6B by 2034 (18% CAGR)
  ([KushoAI summary](https://reports.kusho.ai/state-of-agentic-api-testing-2026)).
  This is a vendor-reported figure, so treat it as directional.

**The existential threat is confirmed.** The Claude Code Stop hook ("block done until
tests pass") is now mainstream advice. Boris Cherny says a verification loop "will 2–3×
the quality" of results, and the canonical hook is ~15 lines of bash
([Brain Bytes](https://codingwithroby.substack.com/p/the-stop-hook-that-wont-let-claude),
[XDA](https://www.xda-developers.com/claude-code-shipping-broken-code-added-automation/)).
**"Run tests before done" is commoditized.** What is not commoditized: detecting that an
API response contract changed **when no test covers it**. That is RegressGuard's only
defensible delta, and all messaging should lead with it.

## 3. PM view

**What to fix (ordered):**
1. **P0: rename the binary to `regressguard`.** `docs/binary-name-decision.md` already
   recommends this. Its absence is now proven to break the primary install path.
   - Change `.mcp.json`, `install.sh`, `action.yml`, README and the hook in one PR.
   - Republish the registry entry at the new version.
2. **P0: the plugin must work without a separate binary install.** The `.mcpb` bundles
   binaries; the Claude Code plugin does not. Ship the binary with the plugin, or run an
   install-on-first-use script. Target: plugin install → first `check` in under 90s
   (plan item 2.3).
3. **P0: make the gate measurable.** Add an opt-in anonymous weekly ping
   (repo hash + command + version), off by default and announced in `rg init`.
   Without it, "≥25 repos with weekly use" can't be scored.
4. **P1: ship as a Stop hook.** `regressguard hook install --claude` should write the
   Stop hook config, so the agent can't declare "done" while a contract regressed.
   This rides harness absorption instead of fighting it.
5. **P1: proof line and PR badge (plan 2.2).** This gives users a shareable artifact.
6. **Deferred:** array union schema (1.2), Windows, Python/FastAPI, the team baseline.
   Each should wait for a user who asks for it.

**Don't build:** dashboards, AI test generation, record/replay. The reasons in
`plan-10x-2026-07.md` §non-goals still hold.

## 4. CEO view

- **Category:** right problem at the right time. DORA's "AI raises instability"
  finding is the whole pitch, backed by Google data.
- **Scale honesty:** as scoped, this is feature-sized. The venture-scale players
  (CodeRabbit, Momentic) sell to teams with hosted products. A local CLI with no hosted
  surface caps out at "respected OSS tool" [assumption, consistent with the July analysis].
- **What would make it a company:** owning the verification record for AI-authored
  changes. This is the org-level "did every agent PR pass a contract gate?" layer, i.e.
  the paid layer in `paid-layer-spec.md`. Only worth building after ~10 teams use the
  free tool.
- **Founder allocation:** the product is ahead of the distribution by ~3 months. The
  next 30 days should be ~20% code and ~80% distribution and user conversations.

## 5. CFO view

**Unit math to reach an indie outcome of $5k MRR** [assumption-driven, benchmarks sourced]:

| Input | Value | Basis |
|---|---|---|
| Price | $20/dev/mo, ~5 devs/team → $100/team/mo | `paid-layer-spec` hypothesis; CodeAnt charges $24/user/mo flat |
| Paying teams needed | 50 | $5k ÷ $100 |
| OSS→paid conversion | 1–3% (devtools), 0.3–1% mass-market OSS | [Monetizely](https://www.getmonetizely.com/articles/whats-the-optimal-conversion-rate-from-free-to-paid-in-open-source-saas) |
| Active free teams required | **~1,700–5,000** | 50 ÷ conversion rate |
| Active free teams today | ~0 | §1 |

**Read:** monetization is 3–4 orders of magnitude away. That is why paid-layer code has
zero ROI right now.

**Per-PR alternative:** Anthropic charges $15–25 per PR for probabilistic review. A
deterministic GitHub App at ~$1–2 per verified PR, or ~$29/repo/mo, is a credible
complement. At $29/repo, 170 repos gets to $5k MRR. That needs fewer adopters than the
per-seat model [assumption; needs validation].

**Burn:** ~$0 plus the maintainer's time. Keep it that way. The real cost is opportunity
cost, which is why the 30-day gate needs a hard stop.

## 6. CMO view

- **Positioning:** "AI reviewing AI is an echo chamber. RegressGuard runs your app."
  - Lead with **"catches API contract breaks your tests don't cover"**, not "regression
    testing".
  - This counters both the Stop-hook commoditization and CodeRabbit's category.
- **ICP:** Claude Code / Cursor power users building JS APIs. That group is large
  (2.6M commits/week from Claude Code alone). Secondary: agencies shipping client APIs
  with agents.
- **Channels, in order of expected yield** [assumption; the gate will measure it]:
  1. Show HN with the break→detect→self-fix GIF (never done).
  2. Claude Code plugin marketplaces and awesome-lists (PRs).
  3. One postmortem-style post, e.g. "My agent deleted a response field. Tests passed."
     Pair it with the DORA instability chart.
  4. Replies in threads where people share Stop-hook setups: "here's the contract layer
     on top".
  5. GitHub Action marketplace.
- **Viral artifact:** the "Commit blocked: `role` removed from GET /api/users"
  screenshot and the PR proof badge.

## 7. CTO view

- **Code quality:** high. The FP discipline, HMAC snapshots and per-test identity are
  real strengths.
- **Architectural debts:**
  - Binary name collision (P0, see §3).
  - No telemetry.
  - Unix-only syscalls in `checkrun` (no Windows).
  - First-element array schema.
  - Single-dev local baseline.
- **Keep:** zero-LLM determinism. It is the differentiation against CodeRabbit and
  Anthropic Code Review, which are probabilistic.
- **Reusable asset for any pivot:** the black-box route prober + schema normalizer +
  dynamic-field stripper. It works against any HTTP surface, including a deployed URL.

## 8. Continue or pivot

| Option | Evidence for | Evidence against | Build cost | Verdict |
|---|---|---|---|---|
| **A. Continue: fix onboarding, launch properly, Stop-hook positioning** | DORA instability; 17M agent PRs/mo; the gate was never tested | Stop hooks commoditize "tests pass"; crowded shelf | ~5 days | **Do now** |
| B. Hosted PR verification GitHub App (deterministic, per-repo or per-PR) | $15–25/PR anchor; CodeRabbit $50M ARR shows teams pay per repo/PR | Needs hosted infra + GitHub App; competes for the same PR surface | 3–4 wks | **First pivot candidate** if A passes the usage bar but not the $ bar |
| C. Deployed-app contract/exposure monitor for vibe-coded apps (Lovable/Bolt/Replit) | $500M+ ARR platforms; 5k/380k apps leaking; non-technical users can't run CLIs | Security-scanner space filling fast (VibeEval, Bastion); liability; new ICP | 4–6 wks | **Pivot candidate** if A fails; smoke-test with a landing page first |
| D. Generic "verification harness for any agent loop" | Fallback from the July analysis | Stop hooks already are that; no delta | — | **Drop.** The market absorbed it |
| E. Shelve as a portfolio piece | Zero sunk-cost risk | Abandons a validated pain before testing it | 0 | Only if A **and** a C smoke test fail |

**Gate v2 (Oct 1–31, measured via telemetry + GitHub):**
- **Pass:** ≥100 stars, ≥15 repos pinging weekly, and ≥5 user conversations where 3+
  mention the same missing feature. Then build the Stop hook + Python as asked, and open
  paid-layer design-partner talks.
- **Partial:** stars without usage. Positioning works but the product doesn't stick.
  Interview users about why, then consider B.
- **Fail:** <30 stars after a *real* Show HN + 3 channels. Run a 1-week landing-page
  smoke test for C ("paste your app URL, see what broke or leaked since the last
  deploy"; >5% waitlist conversion from ~500 visits → build). Otherwise go to E.

## 9. 30-day plan

| Week | Deliverable |
|---|---|
| 1 (Sep 29–Oct 3) | Rename to `regressguard` · fix `.mcp.json` · plugin bundles or installs the binary · opt-in ping · republish registry + tag v0.2.0 |
| 2 | Show HN (Tue–Thu 9am ET) · blog post · 3 plugin marketplace/awesome-list PRs · `hook install --claude` (Stop hook) |
| 3 | 10 user conversations (DM Show HN commenters and Stop-hook blog authors) · fix the top friction item only |
| 4 | Score Gate v2 · decide A / B / C / E and log the numbers in `docs/LAUNCH.md` |

## Sources
- https://survey.stackoverflow.co/2025/ai
- https://cloud.google.com/blog/products/ai-machine-learning/announcing-the-2025-dora-report
- https://redmonk.com/rstephens/2025/12/18/dora2025/
- https://www.danilchenko.dev/posts/2026-04-11-github-ai-agents-pull-requests/
- https://quasa.io/media/github-s-ai-agent-tsunami-275-million-commits-a-week-14-billion-projected-for-2026-and-the-platform-is-starting-to-crack
- https://sacra.com/c/coderabbit/
- https://techfundingnews.com/coderabbit-lands-143m-at-1-5b-valuation-as-ai-generated-code-surges/
- https://techcrunch.com/2025/12/19/cursor-continues-acquisition-spree-with-graphite-deal
- https://code.claude.com/docs/en/code-review
- https://techcrunch.com/2025/11/24/momentic-raises-15m-to-automate-software-testing/
- https://www.prnewswire.com/news-releases/testsprite-raises-6-7-million-seed-round-to-become-the-testing-backbone-of-the-ai-native-development-era-302598377.html
- https://tracxn.com/d/companies/keploy/__lTKMBFH3EscRafIRVxo_LNmjVc2M3TkfO72VRNMrhrs
- https://reports.kusho.ai/state-of-agentic-api-testing-2026
- https://techcrunch.com/2026/07/08/lovable-reportedly-in-talks-to-double-its-valuation-to-13-2b/
- https://sacra.com/c/replit/
- https://www.superblocks.com/blog/lovable-vulnerabilities
- https://vibegraveyard.ai/story/redaccess-vibe-coded-apps-380k-data-exposure-study/
- https://thenextweb.com/news/lovable-vibe-coding-security-crisis-exposed
- https://codingwithroby.substack.com/p/the-stop-hook-that-wont-let-claude
- https://www.xda-developers.com/claude-code-shipping-broken-code-added-automation/
- https://mcp-marketplace.io/blog/state-of-mcp-monetization-2026
- https://www.digitalapplied.com/blog/mcp-adoption-statistics-2026-model-context-protocol
- https://www.getmonetizely.com/articles/whats-the-optimal-conversion-rate-from-free-to-paid-in-open-source-saas

---

## 10. Addendum (same day): competition, product audit, and the company path

### 10.1 Competition: does anyone do our exact job better? [measured stars via GitHub API, 2026-09-24]

| Tool | Stars / funding | What it does | Better than RG at | Worse than RG at |
|---|---|---|---|---|
| Keploy | 18.5k ★, pushed today | Record/replay traffic plus dependency mocks, production sandboxes | Stateful POST/PUT routes, dependency isolation, breadth | Setup weight (eBPF/Docker); not agent-loop-first |
| Hurl | 19.2k ★ | Plain-text HTTP tests | Mature, explicit assertions | Humans must write every assertion; no baseline diff |
| Schemathesis | 3.6k ★ | Property-based tests from an OpenAPI spec | Fuzzing depth | Needs a spec. AI-built apps rarely have one |
| oasdiff / Optic | 1.4k / 1.5k ★ (Optic idle since Jan) | Diff two OpenAPI specs | Spec-level breaking-change rules | Diffs the spec, not the running app; needs a spec |
| Tusk Drift (YC) | 143 ★ | Live-traffic record/replay, Node SDK | Real traffic as tests | Needs SDK instrumentation and traffic |
| TestSprite | ~$8.1M raised | Cloud LLM-generated tests via MCP | UI + API breadth, plans from PRDs | Cloud account, API key, probabilistic |
| Claude Code Stop hook | free, ~15 lines | Blocks "done" until **your tests** pass | Zero install, already mainstream | Only checks what tests cover |
| CodeRabbit / Anthropic Code Review | $50M ARR / $15–25 per PR | LLM reads the diff | Reasoning about intent | Never executes the app |

**Verdict: nobody does our exact job better.** The job is: zero config, no spec, no
tests written, local, deterministic, catches response-contract drift on the live app,
inside the agent loop. Each competitor wins on one axis by requiring something RG
doesn't: a spec, instrumentation, a cloud account, hand-written tests, or an LLM. The
cheapest substitute is an agent writing `toMatchSnapshot()` API tests itself. RG beats
that only if the baseline is something the agent **cannot** rewrite (see 10.3).

**Relevance: yes, in a narrow niche that is growing fast.** The niche is projects
without OpenAPI specs and without API tests. That describes most AI-built code.

### 10.2 Product audit: does it do what it claims? [measured, this session]

| Check | Result |
|---|---|
| Build, vet, staticcheck, gofmt | Clean after cleanup (7 dead symbols removed, 11 files gofmt'd) |
| Unit tests (13 packages) | All green |
| e2e: field removed | CRITICAL, commit blocked, hint names `route.ts` ✅ |
| e2e: status 401→403 | CRITICAL ✅ |
| e2e: field type string→number | CRITICAL ✅ |
| e2e: value-only change | PASS, no false positive ✅ |
| 5 repeated checks, no change | 5/5 PASS, no flakiness ✅ |
| MCP stdio (initialize, tools/list, check) | Works ✅ |
| e2e: test broken | Detected, but **the test wasn't named**: the vitest parser never matched real output. **Fixed** |
| Snapshot with server down | **Overwrote a 5-route baseline with 0 routes, exit 0. Fixed** (refuses, exit 2) |
| Claude Code plugin | **Broken on ripgrep machines. Fixed** (runs `regressguard`) |
| Demo fixture | **Didn't boot** (unpinned TS → TS 7 crashes Next 14). **Fixed** (pinned + lockfile tracked) |
| CI | **None on push/PR. Added** lint + test + e2e |
| Express support | No tracked e2e fixture; unit tests only. Open |

Pattern: every bug was invisible to unit tests and visible within minutes against a
real app. The e2e CI job is the structural fix.

### 10.3 Solve one problem really well

**The one problem:** *an agent must not be able to report "done" while an API response
contract has changed without human approval.*

The biggest gap against that sentence: **the MCP server exposes `snapshot`, so the
agent can re-baseline its own regression.** `check` fails → agent calls `snapshot` →
`check` passes. The guard can be talked out of guarding. To do the job really well:

1. **The baseline is human-owned.**
   - MCP exposes `check` and `status` by default; `snapshot` only with an explicit
     config opt-in.
   - Baseline changes show up as a reviewable diff of `snapshot.json` in the PR.
   - **Done in PR #4 (2026-09-25):** `snapshot` is hidden from agents unless `mcp.allowSnapshot` is set. Remaining gap: an agent with shell access can still run `rg snapshot`.
2. **Zero false positives on real repos.** Measure it on 10 public Next.js/Express
   repos, not fixtures.
3. **Under 90s from install to first green check.**
4. **Every finding is actionable by the agent:** named field, named test, named file.
   This works now.

Everything else (Python, arrays, timing, dashboards) is secondary to those four.

### 10.4 From feature to company

The pattern to copy [analogy, not data]: open-source visual-snapshot testing plus a
hosted *review-and-approve* workflow for snapshot changes. The diff engine is free; the
approval workflow and history is the paid product.

| Stage | What exists | Proof metric | Revenue |
|---|---|---|---|
| 1. Tool (now → Q4 2026) | CLI + MCP + hook, human-owned baseline | 25+ repos checking weekly; 0 FP reports | $0 |
| 2. Product (after stage-1 gate) | GitHub App: CI-canonical baseline; every contract change becomes an "approve this API change" PR check | 10 teams with the app installed; 3 ask to pay unprompted | $29/repo/mo or per-check, anchored under the $15–25/PR AI review price |
| 3. Company | System of record for AI-authored changes: history, cross-repo view, signed compliance evidence ("every agent PR passed a contract gate") | 3 paying orgs > $500/mo; one compliance-driven buyer | Org tier, annual contracts |

**Why this can become a company:** AI review vendors (CodeRabbit $50M ARR) sell
*opinions* about the diff. The gap in the market is *evidence*: a deterministic,
signed record that the running system kept its contract. Evidence compounds with
history, which becomes the moat. Opinions don't compound.

**What decides success is still distribution.** Stage 1 needs Show HN plus the plugin
listing. The product is now correct enough to launch; before this session it was not.
