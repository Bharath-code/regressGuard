import { NextRequest, NextResponse } from "next/server";

// Demo auth route — returns 200 with a valid token, 401 without.
// Used to demonstrate status code regression detection.
export async function GET(request: NextRequest) {
  const auth = request.headers.get("Authorization");

  if (!auth || !auth.startsWith("Bearer ")) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  return NextResponse.json({
    valid: true,
    user: { name: "Alice", role: "admin" },
  });
}
