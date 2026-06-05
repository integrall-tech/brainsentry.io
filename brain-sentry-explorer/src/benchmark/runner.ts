// brain-sentry retrieval benchmark — a REPRODUCIBLE, self-contained
// measurement of search quality against a known ground truth.
//
// This is NOT LongMemEval/LoCoMo (those need external vendor datasets +
// an LLM judge). It seeds the sales-CRM corpus, runs a fixed query set
// whose relevant memories are known by construction, and reports
// aggregate recall@k / precision@k / MRR / nDCG@k. Numbers are honest
// for the conditions printed in the header (LLM + embedding backend),
// and re-runnable by anyone with `npm run benchmark`.

import { BrainSentryClient } from "../api/client.js";
import { searchMemories } from "../api/memories.js";
import { loadConfig } from "../config.js";
import {
  cleanupSalesCorpus,
  idsFor,
  seedSalesCorpus,
} from "../scenarios/sales-corpus.js";
import {
  type AggregateScore,
  aggregate,
  scoreQuery,
} from "./metrics.js";

// The benchmark query set. Each query names the corpus keys that are
// genuinely relevant to it — the ground truth. Chosen to span exact-name,
// thematic, and cross-account retrieval.
interface BenchQuery {
  query: string;
  relevantKeys: string[];
}
const QUERIES: BenchQuery[] = [
  { query: "Acme", relevantKeys: ["acme-discovery", "acme-demo", "acme-objection-sso", "acme-followup-plan"] },
  { query: "Okta SSO objection", relevantKeys: ["acme-objection-sso"] },
  { query: "Globex Enterprise contract signed", relevantKeys: ["globex-decision-contract"] },
  { query: "Globex", relevantKeys: ["globex-demo", "globex-negotiation", "globex-decision-contract", "globex-onboarding"] },
  { query: "Initech lost to competitor on price", relevantKeys: ["initech-decision-loss"] },
  { query: "Hooli dormant ignored follow-ups", relevantKeys: ["hooli-ghosted"] },
  { query: "pricing discount pattern education", relevantKeys: ["pattern-edu-discount"] },
  { query: "deprecate Starter tier decision", relevantKeys: ["decision-deprecate-starter"] },
  { query: "SOC2 security review optimization", relevantKeys: ["optimization-soc2-early"] },
  { query: "two demos close rate insight", relevantKeys: ["insight-2demo-close"] },
];

export interface BenchmarkReport {
  k: number;
  aggregate: AggregateScore;
  perQuery: { query: string; recall: number; rr: number; p: number; ndcg: number }[];
}

const K = 5;

export async function runBenchmark(client: BrainSentryClient): Promise<BenchmarkReport> {
  const ids = new Map<string, string>();
  await seedSalesCorpus(client, ids);
  try {
    const perQuery: BenchmarkReport["perQuery"] = [];
    const scores = [];
    for (const bq of QUERIES) {
      const call = await searchMemories(client, { query: bq.query, limit: 20 });
      const ranked = Array.isArray((call.data as { results?: unknown[] })?.results)
        ? (call.data as { results: { id: string }[] }).results.map((r) => r.id)
        : [];
      const relevant = idsFor(ids, ...bq.relevantKeys);
      const s = scoreQuery(ranked, relevant, K);
      scores.push(s);
      perQuery.push({
        query: bq.query,
        recall: s.recall,
        rr: s.reciprocalRank,
        p: s.precisionAtK,
        ndcg: s.ndcgAtK,
      });
    }
    return { k: K, aggregate: aggregate(scores, K), perQuery };
  } finally {
    await cleanupSalesCorpus(client, ids);
  }
}

const pct = (v: number) => (v * 100).toFixed(1) + "%";

export async function runBenchmarkCLI(): Promise<number> {
  const cfg = loadConfig();
  const client = new BrainSentryClient(cfg);

  const bold = (s: string) => (process.stdout.isTTY ? `\x1b[1m${s}\x1b[0m` : s);
  const dim = (s: string) => (process.stdout.isTTY ? `\x1b[2m${s}\x1b[0m` : s);

  process.stdout.write(bold("brain-sentry retrieval benchmark\n"));
  process.stdout.write(
    dim(`  target ${cfg.baseUrl}  ·  corpus: sales-CRM (20 memories)  ·  k=${K}\n`),
  );
  process.stdout.write(
    dim("  NOTE: own dataset + ground truth — not LongMemEval/LoCoMo.\n\n"),
  );

  const auth =
    cfg.authMode === "login"
      ? await client.login(cfg.email, cfg.password)
      : await client.demoLogin();
  if (!auth.ok || !client.token) {
    process.stderr.write(`authentication failed: ${auth.error ?? auth.status}\n`);
    return 1;
  }

  const report = await runBenchmark(client);

  for (const q of report.perQuery) {
    process.stdout.write(
      `  ${q.query.padEnd(42).slice(0, 42)}  ` +
        `R@${K} ${pct(q.recall).padStart(6)}  MRR ${q.rr.toFixed(2)}  ` +
        `nDCG ${q.ndcg.toFixed(2)}\n`,
    );
  }

  const a = report.aggregate;
  process.stdout.write("\n" + bold("aggregate over " + a.queries + " queries\n"));
  process.stdout.write(`  Recall@${K}:     ${bold(pct(a.meanRecall))}\n`);
  process.stdout.write(`  Precision@${K}:  ${bold(pct(a.meanPrecisionAtK))}\n`);
  process.stdout.write(`  MRR:          ${bold(a.mrr.toFixed(3))}\n`);
  process.stdout.write(`  nDCG@${K}:       ${bold(a.meanNDCGAtK.toFixed(3))}\n`);

  // Exit non-zero only on a clearly broken retriever, so this can gate
  // CI without being flaky on the hash-embedding fallback.
  return a.meanRecall >= 0.5 ? 0 : 1;
}
