// Pure retrieval-quality metrics for the benchmark runner. These COMPUTE
// scores (the assert.ts helpers throw on a threshold; these return the
// number so we can aggregate across a query set). Fully unit-testable.

/** precision@k = (relevant items in top-k) / k_effective. */
export function precisionAtK(
  ranked: string[],
  relevant: Set<string>,
  k: number,
): number {
  const top = ranked.slice(0, k);
  if (top.length === 0) return 0;
  const hits = top.filter((id) => relevant.has(id)).length;
  return hits / top.length;
}

/** recall = (relevant items found anywhere) / total relevant. */
export function recall(ranked: string[], relevant: Set<string>): number {
  if (relevant.size === 0) return 1; // nothing to find = perfect by convention
  const found = new Set(ranked.filter((id) => relevant.has(id)));
  return found.size / relevant.size;
}

/** Reciprocal rank = 1 / (1-based position of first relevant hit), 0 if none. */
export function reciprocalRank(ranked: string[], relevant: Set<string>): number {
  for (let i = 0; i < ranked.length; i++) {
    if (relevant.has(ranked[i])) return 1 / (i + 1);
  }
  return 0;
}

/**
 * nDCG@k with binary relevance. DCG = sum(rel_i / log2(i+2)); IDCG is the
 * DCG of the ideal ordering (all relevant first). Returns 0..1.
 */
export function ndcgAtK(
  ranked: string[],
  relevant: Set<string>,
  k: number,
): number {
  const top = ranked.slice(0, k);
  let dcg = 0;
  for (let i = 0; i < top.length; i++) {
    if (relevant.has(top[i])) dcg += 1 / Math.log2(i + 2);
  }
  const ideal = Math.min(relevant.size, k);
  let idcg = 0;
  for (let i = 0; i < ideal; i++) idcg += 1 / Math.log2(i + 2);
  return idcg === 0 ? 0 : dcg / idcg;
}

export interface QueryScore {
  precisionAtK: number;
  recall: number;
  reciprocalRank: number;
  ndcgAtK: number;
}

export function scoreQuery(
  ranked: string[],
  relevantIds: string[],
  k: number,
): QueryScore {
  const relevant = new Set(relevantIds);
  return {
    precisionAtK: precisionAtK(ranked, relevant, k),
    recall: recall(ranked, relevant),
    reciprocalRank: reciprocalRank(ranked, relevant),
    ndcgAtK: ndcgAtK(ranked, relevant, k),
  };
}

export interface AggregateScore {
  queries: number;
  k: number;
  meanPrecisionAtK: number;
  meanRecall: number;
  mrr: number; // mean reciprocal rank
  meanNDCGAtK: number;
}

/** Mean of per-query scores — the headline numbers for the report. */
export function aggregate(scores: QueryScore[], k: number): AggregateScore {
  const n = scores.length;
  if (n === 0) {
    return {
      queries: 0,
      k,
      meanPrecisionAtK: 0,
      meanRecall: 0,
      mrr: 0,
      meanNDCGAtK: 0,
    };
  }
  const sum = (sel: (s: QueryScore) => number) =>
    scores.reduce((acc, s) => acc + sel(s), 0) / n;
  return {
    queries: n,
    k,
    meanPrecisionAtK: sum((s) => s.precisionAtK),
    meanRecall: sum((s) => s.recall),
    mrr: sum((s) => s.reciprocalRank),
    meanNDCGAtK: sum((s) => s.ndcgAtK),
  };
}
