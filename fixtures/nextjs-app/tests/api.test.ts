// Fixture tests — these run as part of the demo project's test suite.
// They are intentionally simple so rg snapshot captures a stable baseline.
// To demonstrate a regression: comment out one test or change an assertion.

import { describe, it, expect } from "vitest";

describe("health endpoint shape", () => {
  it("returns status and version fields", () => {
    const response = { status: "ok", version: "1.0.0" };
    expect(response).toHaveProperty("status", "ok");
    expect(response).toHaveProperty("version");
  });
});

describe("users endpoint shape", () => {
  it("returns users array and total count", () => {
    const response = { users: [{ name: "Alice" }], total: 1 };
    expect(Array.isArray(response.users)).toBe(true);
    expect(typeof response.total).toBe("number");
  });
});

describe("profile endpoint shape", () => {
  it("includes subscription field", () => {
    const response = {
      name: "Alice",
      email: "alice@example.com",
      subscription: "pro",
      plan: "annual",
    };
    // This test demonstrates schema regression detection.
    // If the AI removes the subscription field, rg check catches it.
    expect(response).toHaveProperty("subscription");
    expect(response).toHaveProperty("plan");
  });
});

describe("auth verify shape", () => {
  it("returns valid and user fields on success", () => {
    const response = { valid: true, user: { name: "Alice", role: "admin" } };
    expect(response.valid).toBe(true);
    expect(response.user).toHaveProperty("name");
    expect(response.user).toHaveProperty("role");
  });
});
