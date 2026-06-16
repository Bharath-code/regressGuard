# RegressGuard paid layer — one-page spec

_Status: **scoped, not built.** This document exists so the open-core story is credible and
the free/paid boundary is explicit. Nothing here ships without PRD change-control (AGENTS.md)._

## Positioning

RegressGuard is **open-core**. The wedge — agent-native verification via the CLI + MCP
server — is free, MIT, and stays that way. It earns trust and adoption at the individual
developer / single-repo level. Willingness-to-pay lives one layer up, at the **team / org**:
people who need to see verification across many repos and agents, keep history, and prove it
to an auditor. The free tool is the funnel; the paid layer is the system of record.

## The boundary

| | Free (this repo, MIT) | Paid (hosted) |
|---|---|---|
| `rg snapshot` / `rg check` / `rg status` | ✓ | ✓ |
| MCP server (`rg mcp serve`) + agent self-verification | ✓ | ✓ |
| Git hook, `--since` scoping, `--auto-server` | ✓ | ✓ |
| Local audit log + `--json` output | ✓ | ✓ |
| Cross-repo / multi-agent **org dashboard** | — | ✓ |
| **History retention** beyond the latest snapshot | — | ✓ |
| **Compliance export** (signed verification record) | — | ✓ |
| SSO, roles, team policy enforcement | — | ✓ |

Rule of thumb: anything that runs **on one machine for one repo** is free. Anything that
**aggregates across repos/people or persists over time** is paid.

## Paid features (v1 scope)

### 1. Org dashboard
A hosted view that ingests each `rg check --json` run (pushed by CI or the MCP server) and
shows, per org: repos under guard, pass/critical/warning trend over time, which agent/commit
introduced a regression, and the current "is everything green" status. Answers a manager's
question the CLI cannot: _"are our AI-generated changes regressing anything, across all our
services?"_

### 2. History retention
The free tool keeps only the latest baseline. The paid layer stores every run's
[`json-contract.md`](json-contract.md) payload plus snapshot metadata, enabling: regression
timelines per route, "when did this contract last change," and blameless diff history. This is
the durable moat — value compounds with retained data, and it cannot be replicated locally.

### 3. Compliance export
A signed, timestamped verification record ("commit `abc123` was checked against baseline
`X` and produced 0 critical regressions") exportable as PDF/CSV/JSON for SOC2 / change-management
evidence. Builds on the existing HMAC snapshot integrity primitive. Targets regulated teams
shipping AI-authored code who must _prove_ a verification gate existed.

## Architecture sketch (non-binding)

- **Ingest:** the existing `rg check --json` contract is the wire format. CI step or MCP
  server `POST`s each run to a hosted endpoint with an org API key. No new client engine.
- **Storage:** append-only run records keyed by org → repo → commit.
- **Auth:** org API key for ingest; SSO for dashboard.
- The CLI/MCP server stays fully functional offline with zero hosted dependency — the paid
  layer is strictly additive.

## Pricing hypothesis (to validate, not commit)

Per-seat or per-repo monthly, with a free tier covering small teams. Anchor on the cost of a
single shipped AI regression vs. the price of the dashboard. **Unvalidated** — needs design-
partner interviews before any build, per the staff review's "make the monetization story real"
condition.

## Explicitly out of scope here

Building any of the above. This is a scoping doc. Implementation requires a PRD change-control
update first (AGENTS.md). See [`staff-review-2026-06.md`](staff-review-2026-06.md) P2-1.
