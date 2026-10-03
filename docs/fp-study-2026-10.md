# False-positive study, 2026-10 (T3.1)

Binary: `regressguard` v0.2.1 (released darwin/arm64). 12 public repos cloned, 10 ran, 2 skipped.
Method: `init`, `snapshot`, then `check` three times with no code change. A false positive (FP) is any verdict other than pass/warn.

## Result

**0 FPs in 30 checks.** That number is weaker than it looks: only 4 of the 10 repos that ran captured any route (5 routes total).
The other 6 passed with 0 routes, so they exercised only the test runner and timing, not the diff engine.
Treat this as "no evidence of FPs", not "FP rate is 0".

| Repo | Routes captured | Run 1 | Run 2 | Run 3 | FP | Notes |
|---|---|---|---|---|---|---|
| js-e2e-express-server | 2 | pass | pass | pass | 0 | Tests 3 pass. |
| nodejs-getting-started | 1 | pass | pass | pass | 0 | Tests 2 pass. |
| no-as-a-service | 1 | pass | pass | pass | 0 | Needed `--test-command` (no test script). |
| swr-site | 1 | pass | pass | pass | 0 | Needed `--test-command`. Scanner missed other routes (below). |
| jsonplaceholder | 0 | pass | pass | pass | 0 | json-server app, framework "unknown". Vacuous. |
| node_crash_course | 0 | pass | pass | pass | 0 | Plain `http` server. Needed `--test-command`. Vacuous. |
| next-app-router-playground | 0 | pass | pass | pass | 0 | Only route is an image endpoint. Needed `--test-command`. Vacuous. |
| Next-js-Boilerplate | 0 (1 skipped) | pass | pass | pass | 0 | `/api/counter` is PUT-only. Tests 2 pass. |
| nextjs-portfolio-starter | 0 | pass | pass | pass | 0 | Pages-router site, no API routes. Needed `--test-command`. Vacuous. |
| tailwind-nextjs-starter-blog | 0 | pass | pass | pass | 0 | Dev server had died during the run, so the checks never touched it. Vacuous. |
| next-email-client | n/a | skipped | | | | Needs `POSTGRES_URL`, no local Postgres. |
| precedent | n/a | skipped | | | | Needs Clerk secrets. No `app/api` routes anyway. |

## Setup errors (not FPs, but trust-relevant)

Two repos produced a non-pass result when the dev server was down at snapshot/check time: `snapshot` printed "routes skipped" and `check` exited 2 with "dev server is not responding".
That is an error, not a regression verdict, and exit 2 is the correct code. Both passed after the server was restarted.
For `nodejs-getting-started` the cause may be the repo's own jest test killing a server with SIGTERM. That was not confirmed.

## Findings (none fixed here)

No FP class occurred twice, so no test+fix was due under T3.1. These gaps are what the study actually surfaced:

1. **`init` hard-fails with no test script** ("no test command configured"). 6 of 10 repos needed a dummy `--test-command`.
   The tests were then never exercised on those repos.
2. **Route discovery misses real routes:**
   - plain `http` servers and json-server apps (0 routes);
   - non-GET-only routes (PUT-only skipped);
   - image routes;
   - swr-site: found `/api/search`, missed `/api/chat` and `[lang]` route handlers (`rss.xml`, `llms.txt`, `agents.md`);
   - tailwind-nextjs-starter-blog: found nothing although `app/api/newsletter/route.ts` exports GET and POST.
3. **Scanner mislabels** a pages-router/Nextra site as `nextjs-app-router`.
4. Repos with 0 routes pass with a warning ("API contract not protected"). That is honest, but a user who ignores the warning has no protection.

Items 1 and 2 are the likeliest first-run friction for real users. Whether to fix them, or touch json-server/plain-`http` support at all (a stack-scope question under AGENTS.md), is a PRD change-control decision.

## Next

A real FP rate needs repos that capture routes and have a working test command. Next candidates: Express apps with `app.get(...)` routes and a test script, and Next.js app-router repos with several `route.ts` GET handlers.

## Follow-up fixes (post-study)

Fixed in the next release, each with a failing test first: `init` without a test script (route-only mode),
`route.tsx/.js/.mjs` files, re-exported and destructured handler exports (newsletter, `/api/chat`), and the pages-router mislabel.
Re-verified on the study repos: next-app-router-playground now captures `/api/og` and stays stable over 3 checks;
tailwind-nextjs-starter-blog now finds GET and POST `/api/newsletter`; swr-site now finds `/api/chat`.

Deliberately not fixed:
- plain `http` and json-server apps: new stack support, frozen by AGENTS.md until a PRD change;
- Next.js pages-router API discovery: same reason;
- PUT/POST/PATCH routes are discovered but skipped at hit time unless a request body is configured (`requiresBody` in `internal/engine`), so they are not protected by default;
- content routes outside `app/api` such as `[lang]/rss.xml`: not API routes and need dynamic params.
