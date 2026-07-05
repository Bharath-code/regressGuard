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
  [ -n "${DEV_PID:-}" ] && kill "$DEV_PID" 2>/dev/null || true
}
trap cleanup EXIT

if ! curl -sf -m 2 http://localhost:3000/api/health > /dev/null; then
  echo "Starting fixture dev server..."
  npm run dev > /dev/null 2>&1 &
  DEV_PID=$!
  for _ in $(seq 1 30); do
    curl -sf -m 2 http://localhost:3000/api/health > /dev/null && break
    sleep 1
  done
  # Warm the lazily-compiled routes so the snapshot probe doesn't time out.
  for p in profile users auth/verify; do curl -sf "http://localhost:3000/api/$p" > /dev/null || true; done
fi

echo "── 1. Record the known-good baseline ──────────────────"
"$RG" snapshot

echo
echo "── 2. An AI agent 'improves' the code (silently drops a field) ──"
sed -i '' '/subscription: "pro",/d' app/api/profile/route.ts

echo
echo "── 3. rg check catches it before the commit ───────────"
"$RG" check || true

echo
echo "── 4. The agent-facing payload names the culprit file ─"
"$RG" check --json 2>/dev/null | python3 -c 'import json,sys; f=json.load(sys.stdin)["results"][0]; print("hint:", f["hint"])' || true

echo
echo "── 5. Agent fixes it, check goes green ────────────────"
git -C "$ROOT" checkout -- fixtures/nextjs-app/app/api/profile/route.ts
"$RG" check
