// Unit tests for the pure retrieval metrics. Run with `npm test`
// (node:test via tsx) — no backend required.

import { test } from "node:test";
import assert from "node:assert/strict";
import {
  aggregate,
  ndcgAtK,
  precisionAtK,
  recall,
  reciprocalRank,
  scoreQuery,
} from "./metrics.js";

test("precisionAtK: perfect top-k", () => {
  assert.equal(precisionAtK(["a", "b", "c"], new Set(["a", "b", "c"]), 3), 1);
});

test("precisionAtK: half relevant", () => {
  assert.equal(precisionAtK(["a", "x", "b", "y"], new Set(["a", "b"]), 4), 0.5);
});

test("precisionAtK: empty ranked is 0", () => {
  assert.equal(precisionAtK([], new Set(["a"]), 5), 0);
});

test("recall: finds all", () => {
  assert.equal(recall(["a", "b", "c"], new Set(["a", "c"])), 1);
});

test("recall: finds half", () => {
  assert.equal(recall(["a", "x"], new Set(["a", "b"])), 0.5);
});

test("recall: empty relevant is perfect by convention", () => {
  assert.equal(recall(["a"], new Set()), 1);
});

test("reciprocalRank: first position", () => {
  assert.equal(reciprocalRank(["a", "b"], new Set(["a"])), 1);
});

test("reciprocalRank: second position", () => {
  assert.equal(reciprocalRank(["x", "a"], new Set(["a"])), 0.5);
});

test("reciprocalRank: not found", () => {
  assert.equal(reciprocalRank(["x", "y"], new Set(["a"])), 0);
});

test("ndcgAtK: ideal ordering is 1.0", () => {
  assert.equal(ndcgAtK(["a", "b"], new Set(["a", "b"]), 5), 1);
});

test("ndcgAtK: relevant lower ranked < 1", () => {
  const score = ndcgAtK(["x", "a"], new Set(["a"]), 5);
  assert.ok(score > 0 && score < 1, `expected 0<score<1, got ${score}`);
});

test("ndcgAtK: nothing relevant is 0", () => {
  assert.equal(ndcgAtK(["x", "y"], new Set(["a"]), 5), 0);
});

test("scoreQuery: bundles all four metrics", () => {
  const s = scoreQuery(["a", "x"], ["a"], 5);
  assert.equal(s.reciprocalRank, 1);
  assert.equal(s.recall, 1);
  assert.equal(s.precisionAtK, 0.5);
  assert.ok(s.ndcgAtK > 0);
});

test("aggregate: means across queries", () => {
  const a = aggregate(
    [
      { precisionAtK: 1, recall: 1, reciprocalRank: 1, ndcgAtK: 1 },
      { precisionAtK: 0, recall: 0, reciprocalRank: 0, ndcgAtK: 0 },
    ],
    5,
  );
  assert.equal(a.queries, 2);
  assert.equal(a.mrr, 0.5);
  assert.equal(a.meanRecall, 0.5);
  assert.equal(a.meanPrecisionAtK, 0.5);
});

test("aggregate: empty set yields zeros, no NaN", () => {
  const a = aggregate([], 5);
  assert.equal(a.queries, 0);
  assert.equal(a.mrr, 0);
  assert.equal(a.meanNDCGAtK, 0);
});
