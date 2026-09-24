#!/usr/bin/env bash
# Runs the break→detect→fix→green demo against fixtures/nextjs-app.
# Prereqs: `go build -o rg ./cmd/rg` at repo root; node deps installed in the fixture.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RG="$ROOT/rg"
FIXTURE="$ROOT/fixtures/nextjs-app"
cd "$FIXTURE"

cleanup() {
  git -C "$ROOT" checkout -- fixtures/nextjs-app/app/api/profile/route.ts 2>/dev/null || true
  # npm spawns next as a child; kill both or the server outlives the demo.
  [ -n "${DEV_PID:-}" ] && { pkill -P "$DEV_PID" 2>/dev/null; kill "$DEV_PID" 2>/dev/null; } || true
}
trap cleanup EXIT

if ! curl -sf -m 2 http://localhost:3000/api/health > /dev/null; then
  echo "Starting fixture dev server..."
  DEV_LOG="$(mktemp)"
  npm run dev > "$DEV_LOG" 2>&1 &
  DEV_PID=$!
  up=""
  for _ in $(seq 1 90); do
    curl -sf -m 2 http://localhost:3000/api/health > /dev/null && { up=1; break; }
    sleep 1
  done
  if [ -z "$up" ]; then
    echo "FAIL: dev server did not come up in 90s. Log:" >&2
    cat "$DEV_LOG" >&2
    exit 1
  fi
  # Warm the lazily-compiled routes so the snapshot probe doesn't time out.
  for p in profile users auth/verify; do curl -sf "http://localhost:3000/api/$p" > /dev/null || true; done
fi

# .regressguard/ is gitignored, so fresh checkouts (CI) have no config yet.
[ -f .regressguard/config.json ] || "$RG" init --yes

echo "── 1. Record the known-good baseline ──────────────────"
"$RG" snapshot

echo
echo "── 2. An AI agent 'improves' the code (silently drops a field) ──"
sed -i.bak '/subscription: "pro",/d' app/api/profile/route.ts && rm -f app/api/profile/route.ts.bak

echo
echo "── 3. rg check catches it before the commit ───────────"
# Doubles as the e2e smoke test (CI): each step asserts, so a broken core fails loudly.
if "$RG" check; then echo "FAIL: regression not detected" >&2; exit 1; fi

echo
echo "── 4. The agent-facing payload names the culprit file ─"
{ "$RG" check --json 2>/dev/null || true; } | python3 -c 'import json,sys; f=json.load(sys.stdin)["results"][0]; assert "route.ts" in f["hint"], f; print("hint:", f["hint"])' 

echo
echo "── 5. Agent fixes it, check goes green ────────────────"
git -C "$ROOT" checkout -- fixtures/nextjs-app/app/api/profile/route.ts
"$RG" check
