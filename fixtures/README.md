# Fixtures

This directory contains example projects for RegressGuard demos and integration testing.

## nextjs-app

A minimal Next.js 14 App Router project with four API routes:

| Route | Purpose |
|---|---|
| `GET /api/health` | Basic health check — stable shape |
| `GET /api/users` | User list — demonstrates array schema detection |
| `GET /api/profile` | User profile — demonstrates field removal detection |
| `GET /api/auth/verify` | Auth-gated route — demonstrates status code regression |

### Demo walkthrough

**Setup**

```sh
cd fixtures/nextjs-app
npm install
npm run dev   # starts on http://localhost:3000
```

**Record baseline**

```sh
rg snapshot
```

**Introduce a regression (simulate AI edit)**

Open `app/api/profile/route.ts` and remove the `subscription` field:

```ts
// Before
return NextResponse.json({ name, email, subscription, plan });

// After (AI removed subscription)
return NextResponse.json({ name, email, plan });
```

**Detect the regression**

```sh
rg check
```

Expected output:

```
Check

X 1 regression detected

  Route                                 Before    After     Change
  GET /api/profile                      <hash>    <hash>    schema

Likely cause:
  Response shape changed — a field may have been removed or renamed.

Next:
  rg check --verbose
  git diff

Commit blocked.
```

**Restore and verify clean**

Revert the change, then:

```sh
rg check
# OK No regressions detected
```

### Auth demo

The `/api/auth/verify` route requires a Bearer token. The config includes `demo-test-token` as the test token. To demonstrate a status code regression, remove the auth check from the route handler — `rg check` will catch the `200 → 401` change.

### Pre-commit hook demo

```sh
rg hook install
git add .
git commit -m "test regression"
# RegressGuard blocks the commit if rg check finds critical regressions
```
