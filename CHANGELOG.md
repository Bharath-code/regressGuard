# Changelog

All notable changes to RegressGuard are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versioning follows [SemVer](https://semver.org/).

## [Unreleased]

### Fixed

- `rg snapshot` and `rg check` rendered a green "0 captured"/"0 unchanged"
  routes line on a zero-route snapshot, implying the API contract was
  protected when nothing was actually being checked. Now warns explicitly
  ("API contract not protected") on unsupported/uncaptured stacks.

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

[Unreleased]: https://github.com/Bharath-code/regressguard/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/Bharath-code/regressguard/releases/tag/v0.1.0
