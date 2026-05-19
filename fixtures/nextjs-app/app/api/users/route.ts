import { NextResponse } from "next/server";

// Simulated user store — stable shape for regression testing.
const users = [
  { name: "Alice", email: "alice@example.com", role: "admin", subscription: "pro" },
  { name: "Bob", email: "bob@example.com", role: "user", subscription: "free" },
];

export async function GET() {
  return NextResponse.json({ users, total: users.length });
}
