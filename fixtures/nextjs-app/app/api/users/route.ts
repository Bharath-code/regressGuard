import { NextResponse } from "next/server";

// Simulated user store — stable shape for regression testing.
const users = [
  { name: "Alice", email: "alice@example.com", role: "admin", subscription: "pro" },
  { name: "Bob", email: "bob@example.com", role: "user", subscription: "free" },
];

export async function GET() {
  return NextResponse.json({ users, total: users.length });
}

export async function POST(request: Request) {
  const body = await request.json();
  return NextResponse.json({ created: true, name: body.name, email: body.email }, { status: 201 });
}
