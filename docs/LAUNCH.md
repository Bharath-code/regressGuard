# Launch runbook (Phase 3 of `plan-10x-2026-07.md`)

Everything below is prepared in-repo; each step is an external publish that needs your
GitHub account. Order matters — the release must exist before the Action and registry
listings point at it. Target: all four channels live in one sitting (~2h).

## 0. Prerequisites (once)

- [ ] Commit + push current main (includes `demo/demo.gif`, `.claude-plugin/`, `.mcp.json`,
      `server.json`, `action.yml`).
- [ ] Cut a release: `git tag v0.1.0 && git push --tags`, then create a GitHub Release
      with built binaries (darwin/linux, amd64/arm64) so `install.sh` and the Action
      resolve `latest`.

## 1. GitHub Action → Marketplace (~15 min)

`action.yml` is marketplace-ready (branding, inputs/outputs, PR comment).

- [ ] On the GitHub release page for `v0.1.0`, tick **"Publish this Action to the
      GitHub Marketplace"** (requires the repo to have a README and the action.yml at root — both true).
- [ ] Also push a moving major tag so users can pin `@v0`:
      `git tag -f v0 v0.1.0 && git push -f origin v0`
- [ ] Verify the listing renders, then add this usage snippet to the README CI section:

```yaml
- uses: Bharath-code/regressguard@v0
  with:
    server-command: npm run dev
```

## 2. MCP Registry (~20 min)

`server.json` at repo root is the manifest (validate before publishing — the schema
version evolves).

- [ ] `brew install mcp-publisher` (or download from
      github.com/modelcontextprotocol/registry releases)
- [ ] `mcp-publisher login github` (proves ownership of `io.github.bharath-code`)
- [ ] `mcp-publisher publish` (validates `server.json` and submits; fix schema errors it
      reports — the file here targets the 2025-09-29 schema)
- [ ] Confirm at https://registry.modelcontextprotocol.io

## 3. Claude Code plugin (~20 min)

`.claude-plugin/plugin.json` + `.mcp.json` make this repo installable as a plugin: the
MCP server (`rg mcp serve`) is registered automatically on install. Note: users still
need the `rg` binary on PATH (install.sh / brew) — the plugin wires it up, it does not
ship the binary.

- [ ] Test locally: `claude plugin install Bharath-code/regressguard@github` (or add the
      repo as a marketplace: `/plugin marketplace add Bharath-code/regressguard`), then in
      a project: `rg init && rg snapshot`, ask Claude to verify a change — it should call
      the `check` MCP tool.
- [ ] Submit to community plugin directories (search current ones — e.g.
      `anthropics/claude-code` discussions and the popular community marketplace repos —
      each is a PR adding this repo's URL + one-line description).
- [ ] Cursor: no central directory; add the `.cursor/mcp.json` snippet (already in
      README) to any "awesome-mcp-servers" lists via PR.

## 4. Show HN (~30 min, post Tue–Thu ~9am ET)

Title (pick one, keep under 80 chars):

> Show HN: RegressGuard – catches API contracts your AI agent silently broke

Body draft:

> AI coding agents report "done" when the code compiles and their tests pass — then you
> find out a response field vanished. RegressGuard records a known-good baseline
> (tests + routes + response schemas) and diffs after the agent edits: removed field,
> changed status code, newly failing test → commit blocked, with the culprit file as a
> structured hint.
>
> It's deterministic — no LLM judging LLM output, no cloud, one Go binary, MIT. The
> interesting part is the MCP server: the agent calls snapshot/check as tools inside its
> own loop, so it fixes what it broke before you ever see the diff. 15-second demo GIF in
> the README.
>
> Known limitations are documented (array schemas use first-element shape; test identity
> is best-effort by runner output parsing). Would love feedback on what would make you
> trust — or distrust — a tool like this.

- [ ] Post, then stay responsive in the thread for 3–4 hours (HN punishes drive-bys).

## 5. Measure (the 90-day gate)

- [ ] Watch: GitHub stars, clones (Insights → Traffic), Action installs, registry pulls.
- [ ] Gate from `docs/full-analysis-2026-07.md`: ~200 stars AND ≥25 repos with repeat
      weekly `rg check` by early October 2026. Log weekly numbers at the bottom of this
      file.

## Week log

| Week | Stars | Clones/wk | Notes |
|---|---|---|---|
| 2026-07-05 | — | — | pre-launch |
