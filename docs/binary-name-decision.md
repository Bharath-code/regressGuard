# Binary name: `rg` collides with ripgrep — recommendation

**Status:** recommendation only. Renaming is gated behind PRD change-control; no code
change in this doc's scope.

## Problem

RegressGuard installs as `rg`. ripgrep — preinstalled on GitHub runners and present on
most dev machines — also installs as `rg`. Whichever comes first on PATH wins:

- `rg check` in a shell or script silently runs ripgrep (greps for "check", exits 0 on
  match) — the guard appears to pass without ever running.
- Discovered live: a pre-commit hook with bare `rg check` was a no-op on any machine
  with ripgrep ahead of RegressGuard on PATH.

## Mitigations already shipped (2026-07)

- Hook script embeds the absolute binary path and verifies `version` output contains
  "RegressGuard" before running; missing/wrong binary blocks the commit loudly.
- `rg doctor` warns when PATH `rg` is ripgrep and fails on stale bare-`rg` hooks.
- `install.sh` verifies via absolute path and warns on PATH collision.
- `action.yml` pins `$RG_BIN` to the install dir instead of trusting PATH.

These make the *product* safe, but every doc, tutorial, and user muscle-memory
invocation of `rg <cmd>` remains a landmine on ripgrep machines.

## Recommendation

**Rename the binary to `regressguard` as the canonical name in the next minor release.**

- `regressguard` is unambiguous, greppable, and free on Homebrew/apt/PATH.
- Do **not** ship an `rg` alias: the short name is claimed by a vastly more popular
  tool; shipping it recreates the collision we just fixed. If a short form is wanted,
  `rgd` appears unclaimed in Homebrew core and Debian — verify at release time.
- Migration: release under both names for one minor version (`rg` prints a one-line
  deprecation pointer to `regressguard` on startup), update README/install.sh/action.yml
  in the same PR, remove the `rg` artifact the release after.
- Cost: one-time doc churn + brand adjustment. Benefit: eliminates the whole class of
  collision bugs instead of patching call sites forever.

## Decision needed

PRD change-control sign-off on: rename target (`regressguard`), whether to offer `rgd`,
and the deprecation window.
