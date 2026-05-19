import { NextResponse } from "next/server";

// Demo profile route — stable shape for schema regression testing.
// The "subscription" field is intentionally present; removing it
// demonstrates schema hash mismatch detection.
export async function GET() {
  return NextResponse.json({
    name: "Alice",
    email: "alice@example.com",
    subscription: "pro",
    plan: "annual",
  });
}
