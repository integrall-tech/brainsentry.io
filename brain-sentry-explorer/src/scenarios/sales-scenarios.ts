// Sales-CRM validation scenarios — ride on top of the SALES_CORPUS to
// answer the second-order question: "does the API actually retrieve the
// right things for a realistic workload, not just return shapes?"
//
// Each scenario declares its `requires` so the runner can skip it
// honestly when the backend is missing pgvector / FalkorDB / an LLM key.
//
// The corpus-heavy scenarios are bundled into two "walkthrough" scenarios
// that seed the 20-memory corpus ONCE each (one for core, one for LLM
// features) instead of re-seeding per assertion. With LLM enabled this
// cuts ~5 full seeds down to 2, and runtime from ~22 min to ~10 min.

import {
  classifyQuery,
  createMemory,
  egoGraph,
  getMemory,
  intercept,
  listDecisions,
  memoriesByCategory,
  memoriesByImportance,
  recall,
  recordDecision,
  remember,
  searchMemories,
  suggestRelationships,
  tryDeleteMemory,
  createRelationship,
  relationshipsFrom,
  relatedMemories,
} from "../api/memories.js";
import {
  memorySchema,
  searchResponseSchema,
} from "../api/types.js";
import {
  assert,
  assertEq,
  assertStatus,
  expectMRR,
  expectPrecisionAtK,
  expectRecall,
  expectShape,
} from "./assert.js";
import {
  cleanupSalesCorpus,
  idsFor,
  seedSalesCorpus,
  SALES_CORPUS,
} from "./sales-corpus.js";
import type { Scenario } from "./runner.js";

// ───────── 1. Full corpus walkthrough — core (no LLM required) ─────────
//
// Single 20-memory seed shared across every non-LLM assertion: search
// recall quality (precision@k + MRR), category/importance filter purity,
// and a hand-built relationship chain on the Acme account.

const corpusCore: Scenario = {
  id: "sales-corpus-core",
  title: "CRM full corpus — search + filters + relationships (core)",
  description:
    "Seeds the 20-memory sales corpus ONCE and chains every non-LLM " +
    "assertion over it. Saves ~3 redundant seeds vs. the per-scenario layout.",
  steps: [
    {
      name: "seed the 20-memory sales corpus",
      run: async ({ client, vars }) => {
        // Pre-set the map so cleanup can iterate even on partial-seed failure.
        const ids = new Map<string, string>();
        vars.corpus = ids;
        await seedSalesCorpus(client, ids);
      },
    },
    // --- Search recall ---
    {
      name: "search 'Acme' recalls all 4 Acme memories with high precision",
      run: async ({ client, vars }) => {
        const map = vars.corpus as Map<string, string>;
        const call = await searchMemories(client, { query: "Acme", limit: 10 });
        const res = expectShape(call, 200, searchResponseSchema);
        const ranked = res.results.map((r) => ({ id: r.id }));
        const expected = idsFor(
          map,
          "acme-discovery",
          "acme-demo",
          "acme-objection-sso",
          "acme-followup-plan",
        );
        expectRecall(ranked, expected, 0.75, "search 'Acme' recall");
        expectPrecisionAtK(
          ranked,
          expected,
          4,
          0.6,
          "search 'Acme' precision@4",
        );
      },
    },
    {
      name: "search 'Okta SSO objection' top-ranks the SSO memory",
      run: async ({ client, vars }) => {
        const map = vars.corpus as Map<string, string>;
        const call = await searchMemories(client, {
          query: "Okta SSO objection",
          limit: 10,
        });
        const res = expectShape(call, 200, searchResponseSchema);
        const ranked = res.results.map((r) => ({ id: r.id }));
        expectMRR(
          ranked,
          idsFor(map, "acme-objection-sso"),
          0.5,
          "search 'Okta SSO objection' MRR",
        );
      },
    },
    {
      name: "search 'Globex Enterprise contract' top-ranks the decision",
      run: async ({ client, vars }) => {
        const map = vars.corpus as Map<string, string>;
        const call = await searchMemories(client, {
          query: "Globex Enterprise contract signed",
          limit: 10,
        });
        const res = expectShape(call, 200, searchResponseSchema);
        const ranked = res.results.map((r) => ({ id: r.id }));
        expectMRR(
          ranked,
          idsFor(map, "globex-decision-contract"),
          0.5,
          "search 'Globex contract' MRR",
        );
      },
    },
    // --- Filters ---
    {
      name: "by-category/DECISION returns all DECISION seeds and only those",
      run: async ({ client, vars }) => {
        const map = vars.corpus as Map<string, string>;
        const call = await memoriesByCategory(client, "DECISION");
        const rows = expectShape(call, 200, memorySchema.array());
        const ranked = rows.map((r) => ({ id: r.id }));
        const expected = idsFor(
          map,
          "globex-decision-contract",
          "initech-decision-loss",
          "decision-deprecate-starter",
        );
        expectRecall(ranked, expected, 0.8, "by-category/DECISION recall");
        const wrong = rows.filter(
          (r) => r.category && r.category !== "DECISION",
        );
        assertEq(
          wrong.length,
          0,
          "by-category/DECISION leaked non-DECISION rows",
        );
      },
    },
    {
      name: "by-importance/CRITICAL returns the critical seeds",
      run: async ({ client, vars }) => {
        const map = vars.corpus as Map<string, string>;
        const call = await memoriesByImportance(client, "CRITICAL");
        const rows = expectShape(call, 200, memorySchema.array());
        const ranked = rows.map((r) => ({ id: r.id }));
        const expected = idsFor(
          map,
          "acme-objection-sso",
          "globex-decision-contract",
          "initech-decision-loss",
          "insight-2demo-close",
          "decision-deprecate-starter",
        );
        expectRecall(ranked, expected, 0.8, "by-importance/CRITICAL recall");
        const wrong = rows.filter(
          (r) => r.importance && r.importance !== "CRITICAL",
        );
        assertEq(
          wrong.length,
          0,
          "by-importance/CRITICAL leaked non-CRITICAL rows",
        );
      },
    },
    // --- Relationship graph on Acme ---
    {
      name: "link the four Acme memories (discovery → demo → objection → followup)",
      run: async ({ client, vars }) => {
        const map = vars.corpus as Map<string, string>;
        const [a, b, c, d] = idsFor(
          map,
          "acme-discovery",
          "acme-demo",
          "acme-objection-sso",
          "acme-followup-plan",
        );
        for (const [from, to] of [
          [a, b],
          [b, c],
          [c, d],
        ]) {
          const call = await createRelationship(client, from, to, "RELATED_TO");
          assertStatus(call, 201);
        }
      },
    },
    {
      name: "outgoing list from acme-discovery references acme-demo",
      run: async ({ client, vars }) => {
        const map = vars.corpus as Map<string, string>;
        const [discoveryId, demoId] = idsFor(map, "acme-discovery", "acme-demo");
        const call = await relationshipsFrom(client, discoveryId);
        assertStatus(call, 200);
        assert(
          JSON.stringify(call.data).includes(demoId),
          "acme-discovery's outgoing relationships do not reference acme-demo",
        );
      },
    },
    {
      name: "related-memories of acme-demo includes both neighbors",
      run: async ({ client, vars }) => {
        const map = vars.corpus as Map<string, string>;
        const [discoveryId, demoId, objectionId] = idsFor(
          map,
          "acme-discovery",
          "acme-demo",
          "acme-objection-sso",
        );
        const call = await relatedMemories(client, demoId);
        assertStatus(call, 200);
        const blob = JSON.stringify(call.data);
        assert(
          blob.includes(discoveryId) || blob.includes(objectionId),
          "related-memories of acme-demo missed both direct neighbors",
        );
      },
    },
    {
      name: "cleanup: delete the corpus",
      run: async ({ client, vars }) => {
        await cleanupSalesCorpus(client, vars.corpus as Map<string, string>);
      },
    },
  ],
};

// ───────── 2. Full corpus walkthrough — LLM features ─────────
//
// Second 20-memory seed that exercises the endpoints depending on the
// LLM provider: /v1/intercept (deep analysis path) and the async
// /v1/relationships/{id}/suggest (fire-and-forget detection). Kept as
// its own scenario so the core walkthrough still runs when LLM is off.

const corpusLLM: Scenario = {
  id: "sales-corpus-llm",
  title: "CRM full corpus — intercept + async suggest (LLM-required)",
  description:
    "Re-seeds the 20-memory corpus (cleared by the core walkthrough) and " +
    "validates the LLM-driven retrieval endpoints over it.",
  requires: ["llm"],
  steps: [
    {
      name: "seed the 20-memory sales corpus",
      run: async ({ client, vars }) => {
        // Pre-set the map so cleanup can iterate even on partial-seed failure.
        const ids = new Map<string, string>();
        vars.corpus = ids;
        await seedSalesCorpus(client, ids);
      },
    },
    // --- Intercept ---
    {
      name: "intercept returns the documented response envelope",
      run: async ({ client }) => {
        const call = await intercept(client, {
          prompt:
            "I'm preparing a follow-up call with Acme Corp tomorrow. What " +
            "should I be ready to discuss based on prior interactions?",
          forceDeepAnalysis: true,
        });
        assertStatus(call, 200);
        const data = call.data as Record<string, unknown>;
        assert(
          typeof data.enhanced === "boolean",
          "intercept response missing 'enhanced' boolean",
        );
        assert(
          typeof data.originalPrompt === "string",
          "intercept response missing 'originalPrompt' string",
        );
        assert(
          typeof data.latencyMs === "number",
          "intercept response missing 'latencyMs' number",
        );
      },
    },
    {
      name: "intercept enriches with Acme-specific facts from the corpus",
      run: async ({ client }) => {
        const call = await intercept(client, {
          prompt:
            "I'm preparing a follow-up call with Acme Corp tomorrow. Please " +
            "surface what you know about Acme, Maria Santos, and any blockers.",
          forceDeepAnalysis: true,
        });
        assertStatus(call, 200);
        const blob = JSON.stringify(call.data).toLowerCase();
        const markers = ["maria", "okta", "postgres", "ferreira", "santos"];
        const hits = markers.filter((m) => blob.includes(m));
        assert(
          hits.length >= 2,
          `intercept did not surface Acme content from the corpus. ` +
            `Found [${hits.join(",")}] of [${markers.join(",")}] in response. ` +
            `If 0 hits + enhanced=false: check interception.go ` +
            `getMemorySummaries.`,
        );
      },
    },
    // --- Async suggest ---
    {
      name: "suggest on acme-demo accepts the request (async 202)",
      run: async ({ client, vars }) => {
        const map = vars.corpus as Map<string, string>;
        const [demoId] = idsFor(map, "acme-demo");
        const call = await suggestRelationships(client, demoId);
        assertStatus(call, 202);
      },
    },
    {
      name: "after a wait, the async detection links acme-demo to a neighbor",
      run: async ({ client, vars }) => {
        // The detection goroutine makes one LLM call per candidate memory
        // (deepseek-v4-flash ≈ 3-5s/call against a 20-memory corpus), so
        // 15s wasn't enough in practice — 45s gives generous slack.
        await new Promise((r) => setTimeout(r, 45_000));
        const map = vars.corpus as Map<string, string>;
        const [discoveryId, demoId, objectionId, followupId] = idsFor(
          map,
          "acme-discovery",
          "acme-demo",
          "acme-objection-sso",
          "acme-followup-plan",
        );
        const call = await relationshipsFrom(client, demoId);
        assertStatus(call, 200);
        const blob = JSON.stringify(call.data);
        const neighbors = [discoveryId, objectionId, followupId];
        const found = neighbors.some((id) => blob.includes(id));
        assert(
          found,
          `acme-demo's outgoing relationships did not include any Acme ` +
            `neighbor after 45s. Looked for [${neighbors.join(", ")}].`,
        );
      },
    },
    {
      name: "cleanup",
      run: async ({ client, vars }) => {
        await cleanupSalesCorpus(client, vars.corpus as Map<string, string>);
      },
    },
  ],
};

// ───────── 3. Semantic API: remember + recall round-trip ─────────

const semantic: Scenario = {
  id: "sales-semantic",
  title: "CRM — semantic remember/recall round-trip",
  description:
    "POST /v1/remember and POST /v1/recall — the high-level wrappers " +
    "agents use. Recall after remember should find the seeded memory.",
  steps: [
    {
      name: "remember stores a memory and returns memoryId",
      run: async ({ client, vars }) => {
        const tag = `bsx-${Date.now().toString(36)}`;
        vars.tag = tag;
        const call = await remember(client, {
          text:
            `[bs-explorer] Customer X (Acme-like) sales note ${tag}. Discovery ` +
            `call notes about Postgres 15 compatibility concerns and an SSO ` +
            `requirement via Okta that pushes them to Enterprise tier.`,
          tags: ["bs-explorer", tag, "customer:x"],
          category: "CONTEXT",
          importance: "IMPORTANT",
        });
        assertStatus(call, 200, 201);
        const data = call.data as { memoryId?: string };
        assert(
          typeof data.memoryId === "string" && data.memoryId.length > 0,
          "remember did not return a memoryId",
        );
        vars.id = data.memoryId;
      },
    },
    {
      name: "recall finds the remembered memory by content keyword",
      run: async ({ client, vars }) => {
        const call = await recall(client, {
          query: `Postgres 15 Okta ${vars.tag}`,
          limit: 10,
        });
        assertStatus(call, 200);
        const data = call.data as {
          results?: { memoryId?: string }[];
          total?: number;
          strategy?: string;
        };
        assert(Array.isArray(data.results), "recall has no results[] array");
        assert(
          typeof data.strategy === "string",
          "recall did not name a strategy",
        );
        const ids = (data.results ?? []).map((r) => ({ id: r.memoryId ?? "" }));
        expectMRR([...ids], [vars.id as string], 0.5, "recall MRR");
      },
    },
    {
      name: "cleanup: delete the remembered memory",
      run: async ({ client, vars }) => {
        await tryDeleteMemory(client, vars.id as string | undefined);
      },
    },
  ],
};

// ───────── 4. Dedup vs. paraphrase ─────────

const dedupParaphrase: Scenario = {
  id: "sales-dedup-paraphrase",
  title: "CRM — dedup behavior on a paraphrased memory",
  description:
    "Seeds acme-discovery, then POSTs a clear paraphrase of it. Whether " +
    "dedup catches the paraphrase or not, the scenario asserts the behavior " +
    "is deterministic and documents it.",
  steps: [
    {
      name: "seed acme-discovery as the baseline memory",
      run: async ({ client, vars }) => {
        const seed = SALES_CORPUS.find((s) => s.key === "acme-discovery")!;
        const call = await createMemory(client, {
          content: seed.content,
          summary: seed.summary,
          category: seed.category,
          importance: seed.importance,
          tags: seed.tags,
        });
        const m = expectShape(call, 201, memorySchema);
        vars.baselineId = m.id;
      },
    },
    {
      name: "POSTing a tight paraphrase yields a deterministic id",
      run: async ({ client, vars }) => {
        const call = await createMemory(client, {
          content:
            "Acme Corp discovery call on April 12 2026. CTO Maria Santos and " +
            "head of data Lucas Ferreira attended. Their data-science group is " +
            "12 engineers running credit-risk models on Postgres 15. Migration " +
            "to PG16+ is not planned, which Maria flagged as a hard blocker if " +
            "brain-sentry requires pgvector. Budget is tight for FY2026, " +
            "decision expected by end of Q3 (September). Maria is the " +
            "technical decision-maker; CFO Roberto Lima controls the budget.",
          category: "CONTEXT",
          importance: "IMPORTANT",
          tags: ["bs-explorer", "customer:acme", "paraphrase"],
        });
        const m = expectShape(call, 201, memorySchema);
        vars.paraphraseId = m.id;
        const deduped = m.id === vars.baselineId;
        vars.deduped = deduped;
      },
    },
    {
      name: "the resulting state is consistent (dedup or distinct, not partial)",
      run: async ({ client, vars }) => {
        const baseline = await getMemory(client, vars.baselineId as string);
        assertStatus(baseline, 200);
        if (!vars.deduped) {
          const para = await getMemory(client, vars.paraphraseId as string);
          assertStatus(para, 200);
        }
      },
    },
    {
      name: "cleanup",
      run: async ({ client, vars }) => {
        await tryDeleteMemory(client, vars.baselineId as string | undefined);
        if (!vars.deduped) {
          await tryDeleteMemory(client, vars.paraphraseId as string | undefined);
        }
      },
    },
  ],
};

// ───────── 5. Auto-classification (requires LLM) ─────────

const autoClassify: Scenario = {
  id: "sales-auto-classify",
  title: "CRM — auto-classification of category from content",
  description:
    "POST a memory WITHOUT explicit category — content phrased as a clear " +
    "DECISION. Expect the LLM-driven auto-classifier to set category=DECISION.",
  requires: ["llm"],
  steps: [
    {
      name: "POST a decision-shaped memory without category",
      run: async ({ client, vars }) => {
        const tag = `bsx-${Date.now().toString(36)}`;
        const call = await createMemory(client, {
          content:
            `[bs-explorer] We have decided to discontinue support for the ` +
            `legacy v1 client SDK starting Q4 2026. Reasoning: maintenance ` +
            `cost outweighs the 3% of customers still using it. Migration ` +
            `guide to be published by 2026-08-01. Decision tag ${tag}.`,
          tags: ["bs-explorer", tag],
        });
        const m = expectShape(call, 201, memorySchema);
        vars.id = m.id;
      },
    },
    {
      name: "GET reflects an auto-assigned DECISION category",
      run: async ({ client, vars }) => {
        const call = await getMemory(client, vars.id as string);
        const m = expectShape(call, 200, memorySchema);
        assertEq(
          m.category,
          "DECISION",
          "auto-classifier did not infer DECISION from clearly decision-shaped content",
        );
      },
    },
    {
      name: "cleanup",
      run: async ({ client, vars }) => {
        await tryDeleteMemory(client, vars.id as string | undefined);
      },
    },
  ],
};

// ───────── 6. Query router classification (regex, no LLM) ─────────

const router: Scenario = {
  id: "sales-router",
  title: "Query router — classifies sales queries by intent",
  description:
    "POST /v1/router/classify is regex-driven (no LLM). Verifies the " +
    "router returns a strategy and confidence for representative queries.",
  steps: [
    {
      name: "router returns a strategy for a customer-name lookup",
      run: async ({ client }) => {
        const call = await classifyQuery(client, "show me Globex memories");
        assertStatus(call, 200);
        const data = call.data as { strategy?: string; confidence?: number };
        assert(
          typeof data.strategy === "string" && data.strategy.length > 0,
          "router did not return a non-empty strategy",
        );
        assert(
          typeof data.confidence === "number" &&
            data.confidence >= 0 &&
            data.confidence <= 1,
          `router confidence outside [0,1]: ${data.confidence}`,
        );
      },
    },
    {
      name: "router handles a 'find decisions' temporal-ish query",
      run: async ({ client }) => {
        const call = await classifyQuery(
          client,
          "find decisions about pricing from last quarter",
        );
        assertStatus(call, 200);
        const data = call.data as { strategy?: string };
        assert(
          typeof data.strategy === "string" && data.strategy.length > 0,
          "router did not return a non-empty strategy for temporal query",
        );
      },
    },
  ],
};

// ───────── 7. Decisions CRUD (requires pgvector) ─────────

const decisions: Scenario = {
  id: "sales-decisions",
  title: "CRM — Decision record + list (semantica API)",
  description:
    "POST /v1/decisions/ records a structured DECISION for the Globex win, " +
    "then GET /v1/decisions/ lists it back. Requires migration 8 (pgvector).",
  requires: ["pgvector"],
  steps: [
    {
      name: "record a Decision",
      run: async ({ client, vars }) => {
        const call = await recordDecision(client, {
          category: "sales:win",
          scenario:
            "Globex Industries — fintech, evaluating brain-sentry Enterprise",
          reasoning:
            "Chose Enterprise on compliance (PROV-O audit) + tenant isolation; " +
            "approved 15% year-1 discount, list price years 2-3.",
          outcome: "approved",
          confidence: 0.9,
          tags: ["bs-explorer", "customer:globex", "outcome:win"],
        });
        assertStatus(call, 200, 201);
        const data = call.data as { id?: string };
        assert(
          typeof data.id === "string" && data.id.length > 0,
          "record decision did not return an id",
        );
        vars.id = data.id;
      },
    },
    {
      name: "list decisions includes the new record",
      run: async ({ client, vars }) => {
        const call = await listDecisions(client, 50);
        assertStatus(call, 200);
        const blob = JSON.stringify(call.data);
        assert(
          blob.includes(vars.id as string),
          "list decisions did not include the recorded id",
        );
      },
    },
  ],
};

// ───────── 8. Ego-graph (requires FalkorDB) ─────────

const ego: Scenario = {
  id: "sales-ego-graph",
  title: "CRM — ego graph around the Globex contract decision",
  description:
    "After linking Acme memories, fetch the ego graph around acme-discovery " +
    "and assert it returns connected nodes. Requires FalkorDB.",
  requires: ["falkordb"],
  steps: [
    {
      name: "seed corpus and link 3 Acme memories into a chain",
      run: async ({ client, vars }) => {
        const ids = new Map<string, string>();
        vars.corpus = ids;
        await seedSalesCorpus(client, ids);
        const [a, b, c] = idsFor(
          ids,
          "acme-discovery",
          "acme-demo",
          "acme-objection-sso",
        );
        await createRelationship(client, a, b, "RELATED_TO");
        await createRelationship(client, b, c, "RELATED_TO");
      },
    },
    {
      name: "ego of acme-discovery returns the connected neighbors",
      run: async ({ client, vars }) => {
        const map = vars.corpus as Map<string, string>;
        const [discoveryId, demoId] = idsFor(
          map,
          "acme-discovery",
          "acme-demo",
        );
        const call = await egoGraph(client, discoveryId, 2, 30);
        assertStatus(call, 200);
        const blob = JSON.stringify(call.data);
        assert(
          blob.includes(demoId),
          "ego graph of acme-discovery did not include acme-demo",
        );
      },
    },
    {
      name: "cleanup",
      run: async ({ client, vars }) => {
        await cleanupSalesCorpus(client, vars.corpus as Map<string, string>);
      },
    },
  ],
};

export const salesScenarios: Scenario[] = [
  corpusCore,
  corpusLLM,
  semantic,
  dedupParaphrase,
  autoClassify,
  router,
  decisions,
  ego,
];
