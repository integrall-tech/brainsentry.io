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

// --- Retrieval-quality metrics ---
//
// These let scenarios go beyond "the response is well-formed" and assert
// "the response is RELEVANT" — the second-order property that matters for
// real consumers. Each assertion prints actual vs expected so a regression
// is debuggable from the report alone.

/** Object the metrics consume: anything with a stable `id`. */
export interface RankedItem {
  id: string;
}

/**
 * precision@k — of the first k returned items, what fraction are in the
 * expected-relevant set? Catches false positives in the top of a ranked
 * list (e.g. search returns unrelated rows above the right ones).
 */
export function expectPrecisionAtK(
  results: RankedItem[],
  expectedRelevantIds: string[],
  k: number,
  threshold: number,
  label: string,
): void {
  const relevant = new Set(expectedRelevantIds);
  const top = results.slice(0, k);
  const hits = top.filter((r) => relevant.has(r.id));
  const score = top.length === 0 ? 0 : hits.length / top.length;
  if (score < threshold) {
    const wrong = top.filter((r) => !relevant.has(r.id)).map((r) => r.id);
    throw new AssertionError(
      `${label}: precision@${k} = ${score.toFixed(2)} (${hits.length}/${top.length}), ` +
        `expected ≥ ${threshold} — false positives in top-${k}: [${wrong.join(", ")}]`,
    );
  }
}

/**
 * recall — of the expected-relevant items, what fraction appear anywhere
 * in the results? Catches missing rows (false negatives).
 */
export function expectRecall(
  results: RankedItem[],
  expectedRelevantIds: string[],
  threshold: number,
  label: string,
): void {
  if (expectedRelevantIds.length === 0) return;
  const returned = new Set(results.map((r) => r.id));
  const found = expectedRelevantIds.filter((id) => returned.has(id));
  const score = found.length / expectedRelevantIds.length;
  if (score < threshold) {
    const missing = expectedRelevantIds.filter((id) => !returned.has(id));
    throw new AssertionError(
      `${label}: recall = ${score.toFixed(2)} (${found.length}/${expectedRelevantIds.length}), ` +
        `expected ≥ ${threshold} — missing from results: [${missing.join(", ")}]`,
    );
  }
}

/**
 * Mean Reciprocal Rank — 1/position of the first relevant hit (1.0 means
 * first result is right; 0.5 means second; 0 means not in the list).
 * Best metric for "the right answer should be at the top".
 */
export function expectMRR(
  results: RankedItem[],
  expectedRelevantIds: string[],
  threshold: number,
  label: string,
): void {
  const relevant = new Set(expectedRelevantIds);
  let rr = 0;
  for (let i = 0; i < results.length; i++) {
    if (relevant.has(results[i].id)) {
      rr = 1 / (i + 1);
      break;
    }
  }
  if (rr < threshold) {
    const top3 = results.slice(0, 3).map((r) => r.id);
    throw new AssertionError(
      `${label}: MRR = ${rr.toFixed(2)}, expected ≥ ${threshold} — ` +
        `top-3 returned: [${top3.join(", ")}], expected one of: [${expectedRelevantIds.join(", ")}]`,
    );
  }
}
