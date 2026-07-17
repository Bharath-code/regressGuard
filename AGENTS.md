# AGENTS.md

## Project Source of Truth

Read `RegressGuard-PRD.md` before making changes.

The PRD is the product, UX, architecture, and delivery tracker. Section 11 is the implementation source of truth.

## Required Workflow

1. Pick a task from Section 11.
2. Change its status to `IN PROGRESS`.
3. Implement only that task's scope.
4. Verify acceptance criteria.
5. Record evidence in the PRD or implementation notes.
6. Move status to `DONE` only when the Definition of Done is satisfied.
7. after we features done , create .md document what we have learned why this feature is need . how its used and how it solves problem. this can be used for future reference how product works.

## Tech Stack & Dependencies

- **Language:** Go 1.24+
- **CLI Framework:** [Cobra](https://github.com/spf13/cobra) — command tree, flags, shell completions
- **Interactive Prompts:** [Charm huh](https://github.com/charmbracelet/huh) — guided `rg init` flow (TTY only)
- **Spinners & Progress:** [Charm Bubbles](https://github.com/charmbracelet/bubbles) + [Bubble Tea](https://github.com/charmbracelet/bubbletea) — phase spinners, route progress tables, elapsed timers
- **Terminal Styling:** [Lip Gloss](https://github.com/charmbracelet/lipgloss) — color tokens, banners, styled output
- **TTY Detection:** `mattn/go-isatty` + `charmbracelet/x/term`

## Project Structure

```
cmd/rg/main.go           # CLI entry point
internal/
  cli/                   # Cobra command wiring, help templates
  config/                # Config read/write/validation, env var resolution
  configrun/             # rg config get/set implementation
  checkrun/              # rg check pipeline (load, rerun, diff, render)
  doctorrun/             # rg doctor diagnostics
  engine/                # Core logic: test runner, route hitter, normalizer, diff
  failures/              # Actionable error type (title, cause, next, moreContext)
  hookrun/               # rg hook install/uninstall
  initrun/               # rg init (interactive + non-interactive)
  scanner/               # Project detection, route discovery
  snapshot/              # Snapshot read/write
  snapshotrun/           # rg snapshot pipeline
  state/                 # Local state (.regressguard/state.json) — streaks, flags
  statusrun/             # rg status quick-glance command
  ui/                    # Design system tokens, components, animations
    style.go             # Symbols, colors, MaxWidth, ColorEnabled(), Paint()
    theme.go             # Lip Gloss style definitions
    spinner.go           # Reusable phase spinner (braille dots, stderr, TTY-aware)
    banner.go            # Pass/Warning/Critical banners, celebrations, reveals
    components.go        # ResultLine, Header, Footer, NextSection, TableHeaderRow
    routeprogress.go     # Live route progress table (concurrent updates)
```

## CLI Contracts

- `--json` writes valid JSON to stdout and nothing else.
- Progress, warnings, verbose logs, and diagnostics go to stderr.
- Non-TTY mode must never prompt or hang.
- Respect `NO_COLOR`, `FORCE_COLOR`, and `TERM=dumb`.
- Exit codes:
  - `0`: pass or warnings only
  - `1`: critical regression
  - `2`: usage/config/runtime error

## UX Rules

- Follow Section 6 terminal design system.
- Default screens should fit in one 80-column terminal viewport.
- Every failure must include a clear next command.
- Do not use emoji. Use ASCII symbols: `OK`, `!`, `X`, `i`, `-`, `>`.
- Do not print large route lists or raw responses unless requested with `--verbose`.
- Use `ui.Paint()` for all colored output — it respects TTY/NO_COLOR automatically.
- Use `ui.StaggeredPrint()` for result reveals on TTY (80ms delay per line).
- Use `ui.NewSpinner()` for any operation over 400ms on TTY.
- Spinners write to stderr, never stdout.
- Non-TTY/CI/JSON/hook modes must never emit spinner frames or ANSI escape codes.

## Styling Conventions

- All color usage goes through `internal/ui` — never use raw ANSI codes directly.
- Lip Gloss styles are defined in `ui/theme.go` — reuse existing styles.
- Banners (`PassBanner`, `WarningBanner`, `CriticalBanner`) are for the primary verdict line only.
- `ui.ColorEnabled(writer)` determines if a writer supports color (checks TTY + env vars).
- Animations (stagger, celebration, critical reveal) are TTY-only and auto-disabled elsewhere.

## Error Handling

- All user-facing errors must use `failures.Actionable{}` with:
  - `Title`: what went wrong
  - `Cause`: likely reason
  - `Next`: copy-pasteable command to fix it
  - `MoreContext`: deeper debugging path (usually `--help` or `rg doctor`)
- The CLI layer in `cli.go` renders `Actionable` errors with color when not in JSON mode.
- In `--json` mode, errors are serialized as `{"status":"error","error":{...}}`.

## Testing Conventions

- Use `httptest.NewServer` for route-hitting tests.
- Use `t.TempDir()` for isolated config/snapshot fixtures.
- Use `makeTestScript()` helper to create fake test runners.
- Tests must not depend on TTY — use `bytes.Buffer` for stdout/stderr.
- Slow tests (>5s) should be gated with `if testing.Short() { t.Skip(...) }`.
- Run `go test -short ./...` for fast iteration; full suite for CI.

## Release & MCP Registry Publish

The server is listed on the official MCP registry as `io.github.Bharath-code/regressguard`
(namespace is case-sensitive, must match the GitHub username).

**Automated:** pushing a `v*` tag triggers `.github/workflows/release.yml`, which runs
tests, GoReleaser (GitHub release + tarballs), builds the MCPB bundle from
`mcpb/manifest.json` + `mcpb/run.sh`, smoke-tests it, uploads `regressguard.mcpb`,
rewrites `server.json` (version/identifier/fileSha256) in the workspace, and publishes
to the registry via OIDC (`mcp-publisher login github-oidc`). To release:
`git tag vX.Y.Z && git push origin vX.Y.Z`. The committed `server.json` is a template;
CI stamps the release values at publish time.

**Manual fallback** (if CI is unavailable):

1. Build/upload platform tarballs to the GitHub release (existing flow).
2. Rebuild the MCPB bundle: extract the 4 binaries (darwin/linux × amd64/arm64) into
   `bundle/bin/rg-<os>-<arch>`, alongside `run.sh` (uname-based binary picker that execs
   `rg mcp serve`) and `manifest.json` (MCPB spec 0.3, `server.type: binary`,
   command `/bin/sh ${__dirname}/run.sh`). Bump `version` in `manifest.json`.
3. Pack + validate: `npx -y @anthropic-ai/mcpb pack bundle regressguard.mcpb`.
4. Smoke test: pipe an MCP `initialize` request into `sh bundle/run.sh`, expect a JSON-RPC result.
5. Upload: `gh release upload v<X.Y.Z> regressguard.mcpb`.
6. Update `server.json`: bump both `version` fields, set `identifier` to the new
   `releases/download/v<X.Y.Z>/regressguard.mcpb` URL, set `fileSha256` from
   `openssl dgst -sha256 regressguard.mcpb`. Constraint: `description` <= 100 chars.
7. Publish: `mcp-publisher login github` (device flow) then `mcp-publisher publish`.
   Verify: `curl "https://registry.modelcontextprotocol.io/v0.1/servers?search=regressguard"`.

CI alternative: GitHub Actions OIDC auth removes the device-code step
(registry repo: `docs/modelcontextprotocol-io/github-actions.mdx`).

## Scope Control

v1 is focused on Next.js/TypeScript API regression safety.

Do not add:
- dashboards
- Python support
- visual regression testing
- AI-generated tests
- enterprise compliance features

unless the PRD change-control section is updated first.
