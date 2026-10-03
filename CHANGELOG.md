# Changelog

All notable changes to RegressGuard are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versioning follows [SemVer](https://semver.org/).

## [Unreleased]

### Fixed

- `regressguard init` no longer fails on repos without a test script when API routes were discovered. Tests are skipped and only route contracts are checked; init prints that. With no tests and no routes it still fails, with a clearer message.
- Next.js route discovery now finds `route.tsx/.js/.jsx/.mjs/.mts` files and handlers exported as `export { handler as GET, handler as POST }` or `export const { POST } = factory()`.
- Next.js projects with `pages/` and no `app/` are detected as `nextjs-pages-router` instead of `nextjs-app-router`. Route discovery for the pages router is still not supported.

## [0.2.1] - 2026-10-01

### Fixed

- `regressguard check` no longer rewrites a stale baseline when it passes. The silent refresh let timing drift ratchet into the baseline and let any agent move it by running `check`; baseline changes now only happen via `regressguard snapshot`.
- Release workflow: `concurrency` group so a duplicate tag-push run cancels instead of failing on already-uploaded assets.

## [0.2.0] - 2026-10-01

### Added

- `rg check --base <git-ref>` (and action input `base`, default `auto` on PRs): compare against the snapshot committed at the ref. A working-tree baseline that differs yields `BASELINE_CHANGED` findings and exit 1.

### Changed (breaking)

- **Binary renamed `rg` → `regressguard`** (no `rg` shim: it collided with ripgrep). Release archives are now `regressguard_<version>_<os>_<arch>.tar.gz`. Upgrading: re-run `install.sh` (an old `rg upgrade` cannot find the new archive name), delete the old `rg`, run `regressguard hook install` (hooks from v0.1.x call `rg`; `regressguard doctor` flags them). The installer never deletes the old binary, it only prints a note. The ripgrep PATH check in `doctor` and the hook's ripgrep fallback were removed. The Homebrew line was dropped from the README; the tap formula still carries the old name.

### Security

- The guard no longer tells agents how to re-baseline. Unreadable/incompatible-baseline errors, `rg explain` and stale-baseline `rg status` hints now say a human must approve a baseline change; `next` in JSON/MCP output never names `snapshot`. (First-time "no snapshot found" still points at `rg snapshot`: there is no baseline to bypass yet.)

### Removed

- Snapshot HMAC (`snapshot.hmac`, "integrity warning"). The key was derived from the project path, so it was forgeable and false-alarmed across clones. Use `rg check --base` + CODEOWNERS; `rg doctor` deletes the stale file.

### Changed

- `snapshot.json` is now committed by default: `rg init` writes `.regressguard/*` + `!.regressguard/snapshot.json` to `.gitignore` (`rg doctor` warns on the old whole-directory ignore). Re-snapshotting an unchanged contract no longer rewrites the file, so PRs don't diff on timings/timestamps.
- **Breaking: array schemas now merge object keys across the first 20 elements**
  (was: first element only), so a field dropped from a later item is caught.
  Schema hashes change for routes returning arrays: re-run `rg snapshot` once.
- `rg upgrade` fails closed: a release without `checksums.txt` is refused, and
  the GPG path (which trusted any keyring key) is removed.
- Agent-facing output no longer suggests re-baselining; a human approves baseline changes.
- Bumped `golang.org/x/text` v0.39.0, `golang.org/x/sys` v0.44.0; CI now runs govulncheck + staticcheck; releases attest build provenance.

- **Breaking (MCP): the baseline is now human-owned.** `rg mcp serve` no longer
  exposes the `snapshot` tool by default. An agent that could re-record the
  baseline could accept its own regression (`check` fails → `snapshot` →
  `check` passes). Record baselines with `rg snapshot`, or opt back in with
  `"mcp": {"allowSnapshot": true}` in `.regressguard/config.json`. The `check`
  tool now tells agents to ask the user for intentional changes.
- MCP registration docs use `regressguard` instead of `rg` (ripgrep collision).

### Fixed

- `rg snapshot` and `rg check` rendered a green "0 captured"/"0 unchanged"
  routes line on a zero-route snapshot, implying the API contract was
  protected when nothing was actually being checked. Now warns explicitly
  ("API contract not protected") on unsupported/uncaptured stacks.
- Per-test failure identity never fired for vitest: real vitest output uses
  colored `FAIL file > suite > name` lines, not `×` markers. Findings now name
  the newly failing test instead of "1 new failure(s)".
- `rg snapshot` with the dev server down overwrote a baseline of N routes with
  0 routes and exited 0. It now refuses (exit 2) when a routed baseline exists.
- Claude Code plugin ran bare `rg`, which resolves to ripgrep on most machines,
  so the MCP server never connected. Plugin now runs `regressguard`;
  `install.sh` installs that alias next to `rg`.
- Repair hints could point at the wrong files: changed files were cut to the
  first 5 *before* route matching, so in larger diffs the culprit was dropped.
  Now filters first, then caps the display (`+N more`).
- `rg init`, `rg snapshot` and `rg check` could report a running Next.js dev
  server as down: they probed with `GET /`, which a cold dev server answers
  only after compiling the page (seconds on CI). A first `rg snapshot` right
  after `npm run dev` could save a 0-route baseline. All three now share one
  probe that checks for a listening TCP port.

### Added

- CI on every push/PR: gofmt, go vet, unit tests, and an e2e job that runs
  `demo/demo.sh` (break → detect → fix → green) against the Next.js fixture.
  `demo.sh` now asserts each step instead of swallowing failures.

## [0.1.0] — 2026-07-16

First public release.

### Added

- **Core engine** — `rg snapshot` records a known-good baseline (test results,
  route status codes, normalized response-schema hashes, timings); `rg check`
  re-runs and diffs, blocking on regressions. Deterministic: no LLM, no cloud,
  single Go binary. Exit codes: `0` pass/warn · `1` critical · `2` error.
- **Diff severity rules** — CRITICAL for newly failing tests (compared by test
  *identity* when runner output is parseable, count otherwise), status-code
  changes, breaking schema changes (field removed/changed), and missing routes;
  WARNING for backward-compatible field additions and timing regressions
  (>200ms **and** >50% over baseline).
- **False-positive discipline** — 16 dynamic keys plus ISO-8601/UUID/JWT
  patterns stripped before schema hashing; transient route errors (timeouts,
  connection blips) reported as non-blocking "unverified" warnings, never as
  regressions; server probe retries through dev-server hot-reload stalls.
- **Agent-native MCP server** — `rg mcp serve` exposes `snapshot`, `check`, and
  `status` as tools so agents (Claude Code, Cursor) verify and fix their own
  work inside their loop. Same JSON payload as `rg check --json`
  ([docs/json-contract.md](docs/json-contract.md)); append-only audit log.
- **Repair hints** — every CRITICAL finding carries the changed-since-snapshot
  files plausibly related to the route, so an agent can jump to the culprit.
- **Snapshot integrity** — HMAC-signed baselines; sensitive-field redaction.
- **Workflow surface** — pre-commit hook (`rg hook install`), GitHub Action
  (`Bharath-code/regressguard@v0`) with PR comment, `rg watch`, `rg quickstart`,
  `rg status` (sub-second), `rg explain <route>`, `rg doctor`, `rg upgrade`,
  shell completions.
- **Auto-detection** — `rg init` detects framework (Next.js App Router,
  Express, Hono), test runner (Vitest, Jest, Bun, npm test), package manager,
  and dev-server URL; discovers routes.
- **Calm-by-default UX** — plain, instant output; animations opt-in via
  `--celebrate`; TTY-only color honoring `NO_COLOR`; actionable errors
  (what failed → likely cause → exact next command).
- **Install paths** — `install.sh` (macOS/Linux, amd64/arm64), Homebrew tap,
  prebuilt release binaries with checksums.

### Fixed (during pre-release hardening)

- Pre-commit hook could silently run **ripgrep** instead of RegressGuard when
  both were installed as `rg` — hook now pins an absolute path and verifies the
  binary self-identifies; `rg doctor` flags PATH collisions and stale hooks.
- `rg check` misreported a dev server as down when it was merely stalled by a
  hot-reload recompile immediately after an agent edit — the probe now retries
  briefly; a truly-down server still fails fast.
- Transient route errors no longer surface as CRITICAL "route missing"
  regressions.

### Known limitations (documented trade-offs)

- Test-identity comparison is best-effort (falls back to count comparison when
  runner output can't be parsed).
- Array schemas are inferred from the first element; heterogeneous arrays are
  not flagged.
- Stacks: JS/TS only (Next.js App Router, Express, Hono). Python is planned.

[Unreleased]: https://github.com/Bharath-code/regressguard/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/Bharath-code/regressguard/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/Bharath-code/regressguard/compare/v0.1.1...v0.2.0
[0.1.0]: https://github.com/Bharath-code/regressguard/releases/tag/v0.1.0
