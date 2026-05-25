// Assertion helpers for validation scenarios. A failed assertion throws
// AssertionError; the runner catches it and marks the step failed. This
// keeps scenario code linear and readable — no manual result plumbing.

import type { z } from "zod";
import type { ApiCall } from "../api/client.js";

export class AssertionError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "AssertionError";
  }
}

export function assert(condition: unknown, message: string): asserts condition {
  if (!condition) throw new AssertionError(message);
}

export function assertEq<T>(actual: T, expected: T, label: string): void {
  if (actual !== expected) {
    throw new AssertionError(
      `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
    );
  }
}

/** Assert an HTTP call returned one of the accepted statuses. */
export function assertStatus(call: ApiCall, ...accepted: number[]): void {
  if (!accepted.includes(call.status)) {
    throw new AssertionError(
      `${call.method} ${call.path}: expected status ${accepted.join("|")}, ` +
        `got ${call.status}${call.error ? ` (${call.error})` : ""}`,
    );
  }
}

/**
 * Assert the call succeeded with the given status AND its body matches the
 * zod schema, then return the typed body. This is the workhorse of the
 * scenarios — it catches both wrong statuses and drifted response shapes.
 */
export function expectShape<S extends z.ZodTypeAny>(
  call: ApiCall,
  status: number,
  schema: S,
): z.infer<S> {
  assertStatus(call, status);
  const parsed = schema.safeParse(call.data);
  if (!parsed.success) {
    const issues = parsed.error.issues
      .map((i) => `${i.path.join(".") || "<root>"}: ${i.message}`)
      .join("; ");
    throw new AssertionError(
      `${call.method} ${call.path}: response shape mismatch — ${issues}`,
    );
  }
  return parsed.data;
}
