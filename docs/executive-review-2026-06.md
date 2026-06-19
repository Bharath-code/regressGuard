# RegressGuard — Executive Review

**Date:** 2026-06-19
**Reviewers:** CEO, CFO, CMO, CTO, Staff Engineer lenses
**Status:** Complete analysis

**Codebase reality check:** 8,613 LOC Go, 49 files, 18 test packages, all green. Clean idiomatic Go, sound architecture, MCP server working, open-core paid layer scoped but unbuilt. **Zero real users yet.** E8 (Launch Feedback) and G5/G7 launch gates are NOT STARTED. This is the single most important fact.

---

## 1. Market Viability — Does it solve a real problem?

**Yes, unambiguously.** The problem is one of the best-funded, most acute pains in software right now.

| Signal | Evidence |
|---|---|
| Problem is real | 43% of AI changes need prod debugging; +242.7% incidents/PR; 88% of AI fixes need 2-3 redeploy cycles |
| Market is expanding | AI coding agents growing 50% MoM; ~10M developers; Claude Code = 4.5% of public commits |
| Gap is real | Nobody owns **pre-commit, post-AI-session, agent-native** regression detection for solo/small teams |
| Wedge is defensible | MCP server lets the agent verify its own work inside its loop — no incumbent does this |

**The viability verdict: the problem is real, the wedge is sharp, the timing is now.** The risk is not "is there a market" — it's "can you capture it before Anthropic/Cursor ships this natively."

---

## 2. CEO Perspective — Strategy & Pivots

### What you have right
- **Sharp positioning:** "Before you commit, know what broke." Complementary to Claude Code/Cursor, not competitive. This is correct — do not pick a fight with platforms that could kill you.
- **Right wedge:** MCP/agent-native verification is the moat. The agent self-corrects before showing the human. This is genuinely novel.
- **Right scope discipline:** Next.js/TS only for v1. Good.

### The pivot question
**Do not pivot the product. Pivot the go-to-market.** You have a built product with zero users. The #1 strategic risk is not features or architecture — it's that you have not shipped to humans.

| Decision | Recommendation |
|---|---|
| Pivot? | **No.** The problem/wedge/timing are all correct. |
| Add features? | **No more v1 features.** E13 is DONE. Stop building. Ship to users. |
| Remove features? | Consider removing `rg watch` and `rg explain` from the *marketing surface* — they dilute the "two commands" message. Keep them, but don't lead with them. |
| Real pivot needed | **GTM pivot: from "build" to "distribute."** E8 is the only P0 that matters now. |

### The existential threat
**Anthropic ships `claude code --verify` natively.** This is your biggest risk (correctly identified in PRD §15). Your defense is speed of audience capture, not features. Every day you spend adding features instead of getting users is a day closer to obsolescence.

**CEO directive:** Freeze feature work. Execute E8 (Launch Feedback) this week. 3 real users > 3 new features.

---

## 3. CTO Perspective — Architecture & Technical Moat

### What's world-class already
- Clean package boundaries (`engine`, `checkrun`, `config`, `mcprun`, `ui` separation is excellent)
- Deterministic core (no LLM in the loop — correct for trust)
- stdout/stderr/exit-code contract is rigorous
- HMAC snapshot integrity, env-var secret indirection, MCP audit log — security is thoughtful
- Go single binary + Cobra + Charm — correct stack

### What's not world-class yet

**1. Test coverage gaps (the strategic one is critical):**
| Untested package | Risk |
|---|---|
| `mcprun` (now tested — good) | Was the biggest gap, now closed |
| `snapshot` (integrity.go) | HMAC is a security primitive — must be tested |
| `statusrun`, `doctorrun`, `watchrun`, `upgraderun` | User-facing commands with zero tests |
| `state` | Streak/celebration state logic untested |

**2. The normalizer is the product, and it has a known weakness:** Array schemas inferred from first element only (documented as limitation). Heterogeneous arrays = silent miss. This is acceptable for v1 but is the #1 accuracy risk.

**3. No integration/E2E test:** You have unit tests but no test that runs the full `init → snapshot → AI edit → check → block commit` loop against the fixture project in CI. Add one.

**4. No benchmark:** G5 (false positive benchmark on 5 real projects) is NOT STARTED. **This is a P0 hiding as P1.** Accuracy is the entire product. If the FP rate is >5%, users uninstall. You need this number before launch.

### CTO directives
1. **Add E2E test against `fixtures/nextjs-app`** running the full loop in CI — proves the wedge works end-to-end.
2. **Run the 5-project benchmark now** — document the actual FP rate. This is the one piece of evidence that determines whether you ship.
3. **Test `snapshot/integrity.go`** — HMAC is a trust primitive.
4. **Wire `validatePath` to a real tool arg or remove it** (flagged in staff review, still open).

---

## 4. Staff Engineer Perspective — Code Quality to World-Class

### Code quality: **B+ → A-**

| Dimension | Grade | Note |
|---|---|---|
| Package layout | A | Clean separation of concerns |
| Idiomatic Go | A | Proper error handling, `failures.Actionable` pattern is excellent |
| Testability | B | Core engine well-tested; run-packages under-tested |
| Error UX | A | Actionable errors with next commands — best-in-class |
| Output contract | A | stdout/stderr/JSON discipline is rigorous |
| Concurrency | B+ | Parallel routes with limit; no obvious races |
| Dependency hygiene | A | Minimal, justified deps |
| Observability | C | No structured logging, no metrics, no trace |

### To reach A+ (world-class), do these:
1. **Add a `go vet` + `staticcheck` + `golangci-lint` CI gate.** No linter configured in `.github/`.
2. **Add race detector to CI:** `go test -race ./...`
3. **Fuzz the normalizer.** It parses untrusted JSON from dev servers. `go test -fuzz` the `Normalize` function with random JSON — this is where subtle bugs hide.
4. **Version the snapshot format explicitly.** You have `version: 1` — add a migration story before you need it.
5. **Document the JSON contract stability promise** (you started this in `docs/json-contract.md` — finish the versioning section).
6. **Binary size:** 17.5MB binary. The PRD targeted <16MB. Consider build tags to split MCP/Charm into optional builds.

---

## 5. CFO Perspective — Monetization & Unit Economics

### The honest financial picture
**You have a product with no revenue and no payment integration.** The PRD projects ₹7,560 MRR at Month 12 — that's ~$90/month. This is a side-project scale, not a business. The paid-layer spec is the right idea but it's a spec, not code.

### What's right
- **Open-core is correct.** Free CLI builds trust/audience; paid layer is team/org.
- **Gross margin ~94%** is real — infra cost near zero until ₹2L MRR.
- **Break-even at 5 paid users** — genuinely capital efficient.

### What's wrong
1. **The free tier is too generous.** "3 snapshots/month, 1 project" — but the CLI is fully free and local. There's nothing to gate. The PRD pricing model doesn't match the open-core spec. **Reconcile this.**
2. **Willingness-to-pay is at the team layer, not individual.** The paid-layer spec correctly identifies this, but there's no path from "free CLI user" to "paid team customer" yet.
3. **No payment infrastructure exists.** Stripe/Razorpay are mentioned but not integrated.

### CFO directives — the monetization path
| Phase | Action | Timeline |
|---|---|---|
| Now | **Don't build payments.** Get 100 free users first. | Week 1-4 |
| Validate | Add `rg check --json` → hosted ingest (the wire format exists). Free during pilot. | Month 2-3 |
| First revenue | **Team plan = history retention + org dashboard.** Charge per-repo. | Month 3-4 |
| Scale | Compliance export for regulated teams (SOC2 evidence). Higher ACV. | Month 6+ |

**The key insight:** The durable moat is **history data** (per the paid-layer spec). Local CLI keeps only the latest snapshot. The paid layer stores every run. This compounds — 6 months of regression patterns per codebase can't be replicated. **This is what you charge for.**

**Pricing recommendation:** Don't charge for the CLI. Charge for the hosted history/dashboard at $12/seat/mo or $29/repo/mo. The compliance export is an enterprise add-on at $99+/mo.

---

## 6. CMO Perspective — Marketing & Distribution

### The positioning is good. The distribution is non-existent.

**Current state:** GitHub repo exists, README is strong, install.sh works. Zero posts, zero testimonials, zero users, zero stars (publicly). This is the biggest gap in the entire company.

### The marketing thesis
**"The AI agent broke something and lied about it. Here's how to catch it in 12 seconds."**

This is a fear-driven purchase. Developers who've been burned by silent AI regressions will install this in 60 seconds. Your job is to find them.

### CMO directives — the 7-day distribution sprint
| Day | Action | Why |
|---|---|---|
| 1 | **Screen recording:** AI agent breaks an API route → `rg check` catches it → commit blocked. 60 seconds. Post on X. | This is the tweet that spreads. Fear + proof. |
| 2 | **Indie Hackers post:** "I shipped a CLI that catches AI coding regressions. Here's every decision." | Builders become users. |
| 3 | **r/ClaudeAI, r/cursor, r/webdev, r/SideProject** | Your exact ICP lives here. |
| 4 | **DM 20 warm developers** from git-scope network | Warm outreach, not cold. |
| 5 | **Pin to git-scope README** | Your biggest distribution asset. |
| 6 | **Dev.to/Hashnode:** "Why AI coding agents need a safety net" | SEO for "claude code regression", "cursor broke my code" |
| 7 | **YouTube:** 3-min demo, no editing | Ranks for search terms long-term |

### The content engine (ongoing)
- **Every "saved me" tweet → RT it.** This is your primary social proof.
- **Build in public:** weekly MRR/user thread. Developers follow builders.
- **GitHub README SEO:** optimize for "ai coding safety", "claude code regression test"

### The one marketing mistake to avoid
**Don't market the CLI features. Market the fear.** "Cursor silently reverted my code in March 2026" is a better hook than "schema hash comparison with SHA-256 normalization." Developers don't buy features — they buy safety from embarrassment.

---

## 7. The Gap Analysis — Add, Remove, or Pivot?

### Market gaps that exist
| Gap | Who owns it | Should you fill it? |
|---|---|---|
| Pre-commit AI regression detection | **You** | Yes — this is your wedge |
| Agent-native self-verification (MCP) | **You** | Yes — this is your moat |
| Team/org visibility into AI regressions | Nobody (well) | Yes — this is your revenue |
| Visual/UI regression testing | Chromatic, Percy | **No** — out of scope, don't touch |
| AI-generated test creation | Multiple | **No** — explicitly out of scope |
| Enterprise compliance for AI code | Early stage | **Later** — compliance export is v3 |

### What to REMOVE from the surface
- **Stop leading with `rg watch`, `rg explain`, `rg upgrade`, `rg history`.** They're good features but they dilute the "two commands" message. Mention them in the command table; don't put them in the hero pitch.
- **The PRD's 7-day plan is blown** (you're past 7 days). That's fine — but stop treating it as a constraint. The new constraint is: **ship to users in 7 days, not features.**

### What to ADD (only after users)
1. **FastAPI/Django support** (P2-2 in staff review) — expands TAM 2x. Spec only for now.
2. **Hosted history ingest** — the bridge to revenue.
3. **VS Code extension** (W4) — status bar presence. High retention.

### What NOT to add
- No dashboard yet (v2)
- No Python yet (v2)
- No visual regression (never v1)
- No AI-generated tests (out of scope)

---

## 8. The Brutal Summary

| Question | Answer |
|---|---|
| Is there a market? | **Yes.** One of the best-timed problems in software. |
| Does it solve a real problem? | **Yes.** Silent AI regressions are universal pain. |
| Is the code world-class? | **Close.** B+ → A with lint gate, E2E tests, fuzzing, benchmark. |
| Is the architecture sound? | **Yes.** Clean, deterministic, MCP-native. |
| Should you pivot? | **No.** Pivot from building to distributing. |
| Should you add features? | **No.** Stop. Ship to users. |
| Should you remove features? | **Remove from marketing surface**, not from product. |
| How to market it? | **Fear + proof.** 60-sec screen recording. The tweet that spreads. |
| How to make money? | **Free CLI → hosted history/dashboard → compliance.** Don't charge for the binary. |
| What's the biggest risk? | **Anthropic ships this natively.** Speed of audience capture is your only defense. |
| What's the one thing to do now? | **E8: Get 3 real users this week.** Nothing else matters. |

**The product is built. The problem is real. The moat is MCP. The gap is users. Ship.**

---

## Action Items Priority Queue

| Priority | Action | Owner | Status |
|---|---|---|---|
| P0 | Freeze feature work — no new v1 features | CEO | Pending |
| P0 | Execute E8: get 3 real users this week | CMO | Pending |
| P0 | Run 5-project false positive benchmark (G5) | CTO | Pending |
| P0 | Add E2E test for full init→snapshot→check→block loop | Staff Eng | Pending |
| P1 | Add golangci-lint + race detector CI gate | Staff Eng | Pending |
| P1 | Test `snapshot/integrity.go` (HMAC primitive) | Staff Eng | Pending |
| P1 | Fuzz the normalizer with random JSON | Staff Eng | Pending |
| P1 | Record 60-sec demo video + post on X | CMO | Pending |
| P2 | Reconcile PRD pricing model with open-core spec | CFO | Pending |
| P2 | Spec hosted history ingest (bridge to revenue) | CFO/CTO | Pending |
| P2 | Spec FastAPI/Django route discovery | CTO | Pending |
| P2 | VS Code extension spec | CTO | Pending |
