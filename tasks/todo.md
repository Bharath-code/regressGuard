# Todo

Active plan: checkable items, then a review section when done.

---

# Plan: Field Audit 2026-09-30 → Gate v2 (Oct 1 – Oct 31)

Source: [RegressGuard Field Audit](https://claude.ai/artifact/XLgntcqNhgLhBohAYj2Ynu) (2026-09-30).
Goal: close the trust hatches, launch once properly, and score Gate v2 with real numbers.
Rule: one task in progress at a time. `go test ./...` stays green after every task.

## Audit facts re-verified on `main` (ddc69a2)

| Audit claim | Status on main | Evidence |
|---|---|---|
| PR #4 (human-owned baseline) still open | **STALE**: merged 2026-09-24 | `gh pr list`, commit ad980f9 |
| Registry still at v0.1.0 | TRUE | `server.json:9` → `"version": "0.1.0"`; tags go up to v0.1.1 |
| `.regressguard/` gitignored | TRUE | `.gitignore` line 2 |
| HMAC key = sha256(salt + absPath) | TRUE | `internal/snapshot/integrity.go:84-94` |
| Tamper → warning + "Run: rg snapshot" | TRUE | `internal/checkrun/checkrun.go:91-100` |
| Agent-facing "rg snapshot" hints | TRUE | `checkrun.go:473,484,787` |
| Array diff = element [0] only | TRUE | `internal/engine/schemadiff.go:48-55` |
| Upgrade skips missing checksum; GPG failure continues | TRUE | `internal/upgraderun/upgraderun.go:151-176` |
| x/text v0.23.0, x/sys v0.38.0 | TRUE | `go.mod:47-48` |
| No `--base` ref mode in check | TRUE | no match in checkrun.go |
| Binary still `rg` | TRUE | `.goreleaser.yaml:12`; rename is only a recommendation in `docs/binary-name-decision.md` |

## Decisions (resolved 2026-09-30, logged in PRD §11.16)

- [x] **D1:** Drop the HMAC. Git, CI `--base`, and CODEOWNERS are the baseline authority.
- [x] **D2:** Hard rename `rg` → `regressguard` in v0.2.0 with no `rg` shim, because a shim would bring back the ripgrep collision.
- [x] **D3:** No telemetry. Usage is measured with public proxies: code search for committed `snapshot.json` and Action usage, plus downloads.
- [x] **D4:** `regressguard hooks install --claude` writes a project-scope Stop hook. `init` offers it. Discovery goes through the plugin marketplace listing.

---

## Phase 1: Make it trustworthy (Wk 1, Oct 1–5)

Thesis: an agent must not be able to talk the guard out of guarding. There are 4 hatches, and 1 is closed (MCP snapshot hidden, PR #4).

### T1.1 Remove re-baseline hints from agent-facing output
- **Why:** The guard currently tells the agent how to bypass it (`checkrun.go:95,100,473,484,787`).
- **Change:** Drop the "Run: rg snapshot" / "Consider running rg snapshot" / "If this change is intentional: rg snapshot"
  lines. Replace them with "A human must approve a baseline change (`rg snapshot`, reviewed in PR)" in text
  output only. JSON and MCP `next` fields must never contain `snapshot`.
- **Acceptance:**
  - [x] `grep -rn '"rg snapshot"' internal/checkrun internal/mcprun` returns only human-facing text paths, or nothing.
  - [x] MCP `check` tool result on a CRITICAL diff contains no `snapshot` suggestion (unit test in `mcprun`).
  - [x] `docs/json-contract.md` updated if the `next` field semantics changed.

### T1.2 Remove the HMAC (D1)
- **Change:** Delete `internal/snapshot/integrity.go`, the `WriteHMAC` call after snapshot write, and the
  `VerifyHMAC` block in `checkrun.go:91-97`. `rg doctor` deletes a stale `.regressguard/snapshot.hmac` if present.
  T1.4 replaces it as the anti-tamper mechanism.
- **Acceptance:**
  - [x] `grep -rn "HMAC\|hmac" --include='*.go' .` → no hits outside the doctor cleanup.
  - [x] Same repo cloned to two different paths → identical `check` verdict with no warning.
  - [x] README and `docs/security-features.md` no longer claim snapshot integrity/tamper detection. They point to `--base` + CODEOWNERS.
  - [x] CHANGELOG notes the removal under v0.2.0.

### T1.3 Commit the baseline by default
- **Change:** `.gitignore` → `.regressguard/*` + `!.regressguard/snapshot.json`. `rg init` writes the same
  rule into user repos (audit log and state stay ignored). Existing users get a one-time `rg doctor` hint.
- **Acceptance:**
  - [x] Fresh `rg init && rg snapshot` in a temp git repo → `git status` shows `snapshot.json` as untracked-and-addable, with no audit or state files.
  - [x] `snapshot.json` is byte-stable across two runs on an unchanged API (no timestamps or ordering churn). Otherwise every PR diffs.
  - [x] e2e smoke test in CI still passes.

### T1.4 `rg check --base <git-ref>`: CI compares against the merged baseline
- **Why:** This is the real anti-tamper. The agent can edit the PR's `snapshot.json` but cannot change `origin/main`.
- **Change:** `--base origin/main` loads `snapshot.json` via `git show <ref>:.regressguard/snapshot.json`. If the
  working-tree baseline differs from the ref, emit a distinct result, `BASELINE_CHANGED`: exit 1 in CI,
  with the list of routes whose recorded contract changed. `action.yml` passes `--base` by default on `pull_request`.
- **Acceptance:**
  - [x] Test: PR branch modifies `snapshot.json` to match a regressed API → `--base main` still reports CRITICAL.
  - [x] Test: ref has no snapshot (first adoption) → clear exit 2 message, no panic.
  - [x] `examples/` workflow updated. The README shows "require a human approval when the baseline changes" (CODEOWNERS on `.regressguard/snapshot.json`).
  - [x] JSON contract documents `BASELINE_CHANGED`.

### T1.5 Fail-closed `rg upgrade`
- **Change:** A missing `checksums.txt` is an error. A bad checksum is an error (already true). Remove the GPG path
  (it accepts any keyring key). Add `gh attestation verify` when `gh` is present, or document cosign as a follow-up.
  Release workflow: add `actions/attest-build-provenance`.
- **Acceptance:**
  - [x] Unit tests (httptest server): missing checksums → error; mismatched → error; valid → binary replaced.
  - [x] `upgraderun` coverage > 0% (currently 0%).
  - [x] No code path prints "continuing with checksum only".

### T1.6 Dependency + CI scanner hygiene
- **Change:** `go get golang.org/x/text@v0.39.0 golang.org/x/sys@v0.44.0 && go mod tidy`. Add `govulncheck ./...`
  and `staticcheck ./...` steps to `ci.yml`. Pin the golangci-lint version (the local crash on Go 1.27).
- **Acceptance:**
  - [x] `govulncheck ./...` → "No vulnerabilities found" (including unreachable ones).
  - [x] CI fails on a deliberately introduced staticcheck issue (verify once on a scratch branch, then drop it).

### T1.7 Array schema diff: key union across the first N elements
- **Change:** In `schemadiff.go:52` (and the matching normalizer), merge the object keys of elements `[0..min(N,len))`, N=20,
  before diffing. The schema hash must use the same merged shape, or snapshot and check will disagree.
- **Acceptance:**
  - [x] Test: field removed from item 3 of 5 → CRITICAL `field removed: items[].role`.
  - [x] Test: heterogeneous optional field present in some items → stable, no FP across 2 runs.
  - [x] Existing snapshots: document that re-snapshotting is required (schema-hash change) in CHANGELOG as **breaking**.

### T1.8 Rename + release v0.2.0 + registry republish
- **Depends:** T1.1–T1.7 merged.
- **Change:** Binary `regressguard` only, with no `rg` shim (D2). Update goreleaser, install.sh, action.yml, hook
  script, `.mcpb` manifest, all user-facing strings (`rg check` → `regressguard check`), and the `doctor` ripgrep check. Bump
  `server.json` to 0.2.0. Rebuild `.mcpb` and republish per AGENTS.md "Release & MCP Registry Publish".
- **Acceptance:**
  - [x] `curl` MCP registry API → `io.github.Bharath-code/regressguard` shows 0.2.0.
  - [x] Cold-start e2e (per `docs/e2e-testing-guide.md`) passes on the released binary, not a local build. (2026-10-01: v0.2.0 darwin/arm64, checksum-verified, `demo/demo.sh` exit 0. It still showed the silent stale-baseline refresh, fixed on main post-release, so it ships in the next release.)
  - [x] `grep -rnw 'rg' README.md docs/*.md internal/ --include='*.go' --include='*.md'` → only historical/changelog hits.
  - [x] The installer removes nothing the user owns. It prints one line if an old `rg` from RegressGuard is on PATH.

---

## Phase 2: Launch once, properly (Wk 2, Oct 6–12)

### T2.1 `regressguard hooks install --claude` (Stop hook, D4)
- **Depends:** T1.8.
- **Change:** Writes a Stop hook into project-scope `.claude/settings.json` (merge, don't clobber). `init` offers it
  when `.claude/` exists (TTY only, default yes; skipped in CI and non-TTY). The hook runs
  `regressguard check --json` and exits 2 on CRITICAL, so the agent can't report "done".
- **Acceptance:**
  - [x] Idempotent: running it twice leaves one hook entry.
  - [x] Existing hooks and settings are preserved (golden-file test).
  - [ ] Manual: in Claude Code, break a route → agent Stop blocked with the diff. Fix → Stop allowed. Recorded as GIF.

### T2.2 20-second break→block→fix GIF
- **Acceptance:** [ ] ≤20s, legible at 800px wide, embedded at top of README, same asset used in Show HN.

### T2.3 Show HN
- **Acceptance:**
  - [ ] Posted Tue–Thu, ~9am ET. Headline hook: "Your agent's tests passed. Your API still broke."
  - [ ] Author replies to every comment within 2h on launch day.
  - [ ] Post URL + 48h stats (points, comments, stars delta) logged in `docs/LAUNCH.md`.

### T2.4 Three distribution PRs + one partnership
- **Acceptance:**
  - [ ] PRs opened to 3 lists (awesome-claude-code / hooks / MCP lists). URLs logged in `docs/LAUNCH.md`.
  - [ ] Claude Code plugin marketplace listing (hook + MCP server) submitted.
  - [ ] proof-of-done author contacted with a concrete "tests untouched + contract kept" combined-hook proposal.

---

## Phase 3: Talk to 10 users + measure FP (Wk 3, Oct 13–19)

### T3.1 Public-repo false-positive study
- **Acceptance:**
  - [x] 10 public Next.js/Express repos: snapshot → check ×3 with no code change. (10 ran, 2 skipped; only 4 captured routes, see study.)
  - [x] FP rate published (table: repo, routes, FP count, cause) in `docs/fp-study-2026-10.md`. (0 FPs in 30 checks, but weak evidence: 6 of 10 repos captured 0 routes.)
  - [x] Any FP class with ≥2 occurrences gets a failing test first, then a fix. (None occurred. Route-discovery gaps logged as findings, not fixed.)

### T3.2 10 user conversations
- **Acceptance:**
  - [ ] 10 conversations logged (who, context, job, did they install, top friction), with no names in the repo.
  - [ ] Only the **#1** friction item is fixed this week. Everything else goes to backlog.

---

## Phase 4: Score + decide (Wk 4, Oct 20–31)

### T4.1 Score Gate v2 (pre-committed, no moving goalposts)
| Outcome | Threshold | Action |
|---|---|---|
| Pass | ≥100 stars AND ≥15 public repos with a committed baseline or RG workflow (D3 proxy) AND ≥5 calls with 3+ naming the same need | Spec option B (GitHub App, $29/repo) |
| Partial | Stars without usage | 5 more interviews, then decide B |
| Fail | <30 stars after Show HN + 3 channels | 1-week landing-page smoke test for C (≥5% signup / ~500 visits), else E (shelve) |

- **Acceptance:** [ ] Result, the raw numbers (including the code-search queries for `path:.regressguard filename:snapshot.json` and `"uses: Bharath-code/regressguard"`), and the command/API behind each one logged in `docs/LAUNCH.md` by Oct 31.

---

## Explicitly NOT in this plan
Python/FastAPI/Django, Windows build, dashboards, hosted app (B), splitting `checkrun.go` as its own
task, new strategy docs. These are frozen until Gate v2 passes.

## Review
_(fill in when done)_
