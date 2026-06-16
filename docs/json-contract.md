# `rg check --json` output contract

This is the stable, machine-readable contract emitted by `rg check --json` and returned
verbatim by the MCP `check` tool. It is the integration surface for AI agents today and the
intended ingest payload for a future hosted layer (see [`paid-layer-spec.md`](paid-layer-spec.md)).

Treat this as a versioned API: fields are additive. Removals or semantic changes are breaking
and must bump the snapshot/contract version and be called out in the changelog.

## Top-level object

```json
{
  "status": "pass | warning | critical",
  "summary": { "critical": 0, "warnings": 0, "passed": 0 },
  "results": [ /* CheckFinding objects, see below */ ],
  "next": "git commit"
}
```

| Field | Type | Notes |
|---|---|---|
| `status` | string | `"pass"`, `"warning"`, or `"critical"`. Maps to exit codes: pass/warning → `0`, critical → `1`. |
| `summary.critical` | int | Count of CRITICAL findings. |
| `summary.warnings` | int | Count of WARNING findings (timing + unverified). |
| `summary.passed` | int | Routes that produced **no finding at all**. A route with only a WARNING is *not* counted here. |
| `results` | array | One entry per finding. Empty on a clean pass. |
| `next` | string | Suggested next command for the operator/agent. |

## `CheckFinding` object

```json
{
  "severity": "CRITICAL | WARNING",
  "type": "tests | status | schema | timing | unverified",
  "route": "GET /api/users",
  "before": 200,
  "after": 500,
  "message": "GET /api/users: status 200 -> 500",
  "schemaDiff": [ /* present only when type == "schema" */ ]
}
```

| Field | Type | Notes |
|---|---|---|
| `severity` | string | `"CRITICAL"` or `"WARNING"`. |
| `type` | string | What changed (see table below). |
| `route` | string | Canonical `"METHOD /path"`. Omitted for test-level findings. |
| `before` / `after` | any | Type depends on `type` (see below). Omitted/`null` where not applicable. |
| `message` | string | Human-readable one-liner. |
| `schemaDiff` | array | Only on `type == "schema"`. Field-level changes. |

### `before` / `after` by finding type

| `type` | severity | `before` / `after` | Meaning |
|---|---|---|---|
| `tests` | CRITICAL | int / int (passed counts) | New test failures vs baseline (count delta). |
| `status` | CRITICAL | int / int | HTTP status code changed. A disappeared route uses `before=<status>`, `after=null`. |
| `schema` | CRITICAL | string / string (8-char hash prefixes) | Response shape changed; see `schemaDiff`. |
| `timing` | WARNING | int / int (milliseconds) | Response time regressed (`>200ms` AND `>50%` of baseline). |
| `unverified` | WARNING | int / null | Route could not be measured this run (transient timeout/connection error). **Never blocks** — explicitly not counted as a regression. |

### `schemaDiff` entry

```json
{ "field": "user.email", "action": "removed", "before": "string", "after": "" }
```

| Field | Type | Notes |
|---|---|---|
| `field` | string | Dotted path to the changed field. |
| `action` | string | `"added"`, `"removed"`, or `"changed"`. |
| `before` / `after` | string | Normalized type tokens (e.g. `string`, `number`, `uuid`, `date`). Empty where not applicable. |

## Examples

**Clean pass:**

```json
{
  "status": "pass",
  "summary": { "critical": 0, "warnings": 0, "passed": 6 },
  "results": [],
  "next": "git commit"
}
```

**Critical regression with a concurrent timing warning:**

```json
{
  "status": "critical",
  "summary": { "critical": 1, "warnings": 1, "passed": 4 },
  "results": [
    {
      "severity": "CRITICAL",
      "type": "schema",
      "route": "GET /api/users",
      "before": "a1b2c3d4",
      "after": "e5f6a7b8",
      "message": "GET /api/users: response schema changed",
      "schemaDiff": [
        { "field": "role", "action": "removed", "before": "string" }
      ]
    },
    {
      "severity": "WARNING",
      "type": "timing",
      "route": "GET /api/orders",
      "before": 40,
      "after": 460,
      "message": "GET /api/orders: +420ms slower (40ms -> 460ms)"
    }
  ],
  "next": "rg check --verbose"
}
```

## Ingest notes (for a future hosted layer)

- The payload is self-contained per run; correlate runs by git commit + timestamp captured
  in the snapshot (not in this payload — see `snapshot.json`).
- `summary.passed` excludes warning-only routes by design; do not derive a total route count
  from `passed + warnings + critical` (a route may produce multiple findings).
- `type == "unverified"` is the transient-error signal: ingest may surface it as flapping
  infrastructure, never as a regression metric.
