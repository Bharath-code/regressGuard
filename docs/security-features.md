# Security Features (Section 19.4)

## Why These Features Are Needed

RegressGuard handles sensitive data: auth tokens, API responses, and project configuration. As the tool is used in CI/CD pipelines and by AI agents via MCP, security hardening is essential to prevent:

1. **Secret leakage** — tokens stored in plain text in committed files
2. **Stale credentials** — forgotten tokens that should have been rotated
3. **Snapshot tampering** — modified baselines that hide regressions
4. **Supply-chain attacks** — compromised binaries during self-update
5. **Agent overreach** — MCP-connected agents accessing unrelated directories
6. **Audit gaps** — no visibility into what AI agents are doing via MCP

## Features Implemented

### S1: Token Rotation Warning

**Problem:** Developers set up auth tokens and forget about them. Stale tokens are a security risk — they may have been compromised or should be rotated per policy.

**Solution:** `rg doctor` checks the modification time of `.regressguard/.env`. If the file is older than 30 days, it prints a warning suggesting token rotation.

**Usage:**
```
$ rg doctor
! Token age    .regressguard/.env is 45 days old — consider rotating secrets
  Tip: Update your token and run: touch .regressguard/.env
```

### S2: Snapshot Sanitization (redactFields)

**Problem:** Snapshot files contain normalized schema shapes that may expose internal field names (e.g., `internalUserId`, `adminFlag`, `secretKey`). If `.regressguard/snapshot.json` is committed to a public repo, these names leak.

**Solution:** A `redactFields` config option strips specified field names from `NormalizedSchema` in the snapshot before persisting. Unlike `ignoreFields` (which affects diff comparison), `redactFields` removes fields from the stored snapshot entirely.

**Usage:**
```json
// .regressguard/config.json
{
  "redactFields": ["internalUserId", "adminFlag", "secretKey"]
}
```

```
$ rg config set redactFields "internalUserId,adminFlag"
```

### S3: Config File Permissions

**Problem:** `.regressguard/.env` contains secrets (tokens, cookies). If created with default permissions (0644), any user on the system can read them.

**Solution:**
- `config.WriteEnvFile()` creates `.env` with `0600` (owner-only read/write)
- `rg doctor` checks file permissions and warns if world-readable

**Usage:**
```
$ rg doctor
! Env file     .regressguard/.env has unsafe permissions 0644 — should be 0600
  Fix: chmod 600 .regressguard/.env
```

### S4: MCP Server Directory Restriction

**Problem:** A compromised or misconfigured AI agent connected via MCP could run checks in unrelated directories, potentially accessing sensitive projects.

**Solution:** `rg mcp serve --project-root <dir>` resolves the path to an absolute directory and restricts all operations (check, snapshot, status) to that directory. The path is validated on startup.

**Usage:**
```
$ rg mcp serve --project-root /home/user/my-project
```

In MCP config:
```json
{
  "command": "rg",
  "args": ["mcp", "serve", "--project-root", "/path/to/project"]
}
```

### S5: GPG Signature Verification

**Problem:** SHA-256 checksums verify download integrity but not authenticity. If an attacker compromises the GitHub release, they can replace both the binary and the checksum file.

**Solution:** `rg upgrade` looks for a `.sig` or `.asc` file alongside the release archive. If found and `gpg` is available, it verifies the signature. Failure is non-blocking (warns and continues with checksum-only verification) since not all users have GPG configured.

**Usage:**
```
$ rg upgrade
> Downloading rg 0.2.0 (darwin/arm64)...
> Verifying checksum...
> Verifying GPG signature...
OK Updated: 0.1.0 -> 0.2.0
```

### S6: Audit Log for MCP Calls

**Problem:** When AI agents call RegressGuard via MCP, there's no visibility into what operations were performed, when, or how long they took. This makes debugging agent behavior difficult.

**Solution:** All MCP tool invocations are logged to `.regressguard/mcp-audit.log` as newline-delimited JSON with timestamp, tool name, arguments, status, and duration.

**Log format:**
```json
{"timestamp":"2026-05-23T10:30:00Z","tool":"check","status":"success","durationMs":4200,"args":{"since":"HEAD~1"}}
{"timestamp":"2026-05-23T10:31:00Z","tool":"snapshot","status":"success","durationMs":8100,"args":{}}
{"timestamp":"2026-05-23T10:32:00Z","tool":"status","status":"success","durationMs":45,"args":{}}
```

### S7: Snapshot Integrity Check (HMAC)

**Problem:** If someone manually edits `snapshot.json` (accidentally or maliciously), `rg check` would compare against a tampered baseline, potentially hiding real regressions.

**Solution:** After every `snapshot.Write`, an HMAC-SHA256 is computed over the snapshot content using a project-specific key (derived from the absolute project path + a salt). The HMAC is stored in `.regressguard/snapshot.hmac`. On `rg check`, the HMAC is verified before trusting the snapshot. Mismatch produces a non-blocking warning on stderr.

**Usage:**
```
$ rg check
! Snapshot integrity warning: snapshot.json may have been modified outside of rg snapshot.
  Run: rg snapshot
```

**Design decisions:**
- The key is derived from the project path, not a user secret — this is tamper *detection*, not encryption
- HMAC mismatch is a warning, not a blocker — the user may have legitimate reasons to edit the snapshot
- Snapshots created before S7 don't have an HMAC file; this is handled gracefully (no warning)

## File Changes

| File | Changes |
|------|---------|
| `internal/config/config.go` | Added `RedactFields`, `WriteEnvFile()`, `EnvFilePermissionsOK()`, `EnvFileAge()` |
| `internal/snapshot/snapshot.go` | Updated `Write()` with redaction support, auto-HMAC |
| `internal/snapshot/integrity.go` | New file: HMAC computation and verification |
| `internal/doctorrun/doctorrun.go` | Added S1 (token age) and S3 (permissions) checks |
| `internal/mcprun/mcprun.go` | Added S4 (path validation) and S6 (audit logging) |
| `internal/upgraderun/upgraderun.go` | Added S5 (GPG signature verification) |
| `internal/checkrun/checkrun.go` | Added S7 (HMAC verification warning) |
| `internal/cli/cli.go` | Added `--project-root` flag to `rg mcp serve` |
| `internal/configrun/configrun.go` | Added `redactFields` get/set support |
| `internal/snapshotrun/snapshotrun.go` | Pass `RedactFields` to `snapshot.Write()` |
