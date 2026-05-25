// Validation scenarios for the core memory API. Each scenario chains
// real HTTP calls and asserts that responses are internally consistent
// and shaped as expected — the contract an integrating app depends on.
//
// Every scenario seeds its own data (content prefixed "[bs-explorer]")
// and deletes it in a final cleanup step, so the suite is re-runnable
// and leaves the backend clean.
//
// Important: MemoryService deduplicates by SimHash with a Hamming-distance
// threshold of 3 — two near-duplicate POSTs return the SAME id (the
// existing memory) with status 201, instead of creating a second row.
// Seeds that need to be distinct memories therefore include a 16-hex-char
// random blob via unique(); seeds for the Deduplication scenario
// deliberately stay textually close.

import { randomBytes } from "node:crypto";

import {
  createMemory,
  createRelationship,
  deleteMemory,
  deleteRelationshipBetween,
  feedbackWeight,
  getMemory,
  listMemories,
  memoriesByCategory,
  memoriesByImportance,
  memoryVersions,
  recordFeedback,
  relatedMemories,
  relationshipsFrom,
  searchMemories,
  storeCreate,
  storeDelete,
  storeGet,
  storeSearch,
  tryDeleteMemory,
  updateMemory,
} from "../api/memories.js";
import {
  feedbackWeightSchema,
  memoryListSchema,
  memorySchema,
  searchResponseSchema,
} from "../api/types.js";
import { assert, assertEq, assertStatus, expectShape } from "./assert.js";
import type { Scenario } from "./runner.js";

// Unique-per-run token so seeded content can be found by exact search and
// never collides with a previous run.
function marker(): string {
  return `bsx${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

// 16 hex chars of crypto randomness — long enough to push two seeds well
// past the SimHash dedup threshold of Hamming distance 3.
function unique(): string {
  return randomBytes(8).toString("hex");
}

const num = (v: unknown): number => (typeof v === "number" ? v : 0);

// --- Scenario 1: full CRUD lifecycle ---

const lifecycle: Scenario = {
  id: "lifecycle",
  title: "Memory CRUD lifecycle",
  description:
    "create -> read back -> update -> version history -> delete -> 404. " +
    "Proves create/get/update agree on a memory's content and version.",
  steps: [
    {
      name: "create memory returns 201 with an id",
      run: async ({ client, vars }) => {
        const tag = marker();
        vars.tag = tag;
        const call = await createMemory(client, {
          content: `[bs-explorer] lifecycle subject ${tag}`,
          summary: "lifecycle scenario seed",
          category: "KNOWLEDGE",
          importance: "IMPORTANT",
          tags: ["bs-explorer", tag],
        });
        const m = expectShape(call, 201, memorySchema);
        vars.id = m.id;
        vars.createdVersion = num(m.version) || 1;
      },
    },
    {
      name: "GET reflects the created content, category and importance",
      run: async ({ client, vars }) => {
        const call = await getMemory(client, vars.id as string);
        const m = expectShape(call, 200, memorySchema);
        assert(
          m.content.includes(vars.tag as string),
          `GET content lost the seed marker: "${m.content}"`,
        );
        assertEq(m.category, "KNOWLEDGE", "category round-trip");
        assertEq(m.importance, "IMPORTANT", "importance round-trip");
      },
    },
    {
      name: "PUT updates the content",
      run: async ({ client, vars }) => {
        const call = await updateMemory(client, vars.id as string, {
          content: `[bs-explorer] lifecycle UPDATED ${vars.tag}`,
          changeReason: "bs-explorer validation",
        });
        assertStatus(call, 200);
      },
    },
    {
      name: "the update is persisted and the version advances",
      run: async ({ client, vars }) => {
        const call = await getMemory(client, vars.id as string);
        const m = expectShape(call, 200, memorySchema);
        assert(
          m.content.includes("UPDATED"),
          `update was not persisted: "${m.content}"`,
        );
        if (m.version !== undefined) {
          assert(
            num(m.version) > num(vars.createdVersion),
            `version did not advance: ${vars.createdVersion} -> ${m.version}`,
          );
        }
      },
    },
    {
      name: "version history records the revision",
      run: async ({ client, vars }) => {
        const call = await memoryVersions(client, vars.id as string);
        assertStatus(call, 200);
        assert(Array.isArray(call.data), "versions response is not an array");
        assert(
          (call.data as unknown[]).length >= 1,
          "expected at least one historical version",
        );
      },
    },
    {
      name: "DELETE removes the memory",
      run: async ({ client, vars }) => {
        const call = await deleteMemory(client, vars.id as string);
        assertStatus(call, 200);
      },
    },
    {
      name: "the deleted memory returns 404",
      run: async ({ client, vars }) => {
        const call = await getMemory(client, vars.id as string);
        assertStatus(call, 404);
      },
    },
  ],
};

// --- Scenario 2: search response consistency ---

const search: Scenario = {
  id: "search",
  title: "Search response consistency",
  description:
    "Seeds memories carrying a unique token, then checks the search " +
    "envelope is self-consistent (total == results.length) and finds them.",
  steps: [
    {
      name: "seed three memories with a shared token",
      run: async ({ client, vars }) => {
        const tag = marker();
        vars.tag = tag;
        const ids: string[] = [];
        for (let i = 0; i < 3; i++) {
          // unique() keeps each seed past SimHash dedup; the shared `tag`
          // is what the search query matches.
          const call = await createMemory(client, {
            content: `[bs-explorer] search corpus entry ${i} token ${tag} ${unique()}`,
            category: "REFERENCE",
            tags: ["bs-explorer", tag],
          });
          const m = expectShape(call, 201, memorySchema);
          ids.push(m.id);
        }
        vars.ids = ids;
      },
    },
    {
      name: "search envelope is internally consistent",
      run: async ({ client, vars }) => {
        const call = await searchMemories(client, {
          query: vars.tag as string,
          limit: 10,
        });
        const res = expectShape(call, 200, searchResponseSchema);
        assertEq(res.total, res.results.length, "total vs results.length");
        for (const r of res.results) {
          assert(r.id.length > 0, "search result is missing an id");
        }
      },
    },
    {
      name: "search finds the seeded memories by token",
      run: async ({ client, vars }) => {
        const call = await searchMemories(client, {
          query: vars.tag as string,
          limit: 20,
        });
        const res = expectShape(call, 200, searchResponseSchema);
        const found = res.results.filter((r) =>
          (vars.ids as string[]).includes(r.id),
        );
        assert(
          found.length >= 1,
          `expected to recall a seeded memory; got ${res.results.length} ` +
            `results, none matching the ${(vars.ids as string[]).length} seeds`,
        );
      },
    },
    {
      name: "search rejects an empty query with 400",
      run: async ({ client }) => {
        const call = await searchMemories(client, { query: "" });
        assertStatus(call, 400);
      },
    },
    {
      name: "cleanup: delete seeded memories",
      run: async ({ client, vars }) => {
        for (const id of vars.ids as string[]) {
          await tryDeleteMemory(client, id);
        }
      },
    },
  ],
};

// --- Scenario 3: list pagination ---

const pagination: Scenario = {
  id: "pagination",
  title: "List pagination integrity",
  description:
    "Seeds six memories and checks the paged list envelope: page/size " +
    "echo, totalPages math, hasNext/hasPrevious flags, no row overlap.",
  steps: [
    {
      name: "seed six memories",
      run: async ({ client, vars }) => {
        const tag = marker();
        const ids: string[] = [];
        for (let i = 0; i < 6; i++) {
          // unique() per row defeats SimHash dedup — without it all six
          // POSTs returned the first memory's id and only one row landed.
          const call = await createMemory(client, {
            content: `[bs-explorer] pagination row ${i} ${tag} ${unique()}`,
            category: "CONTEXT",
            tags: ["bs-explorer", tag],
          });
          const m = expectShape(call, 201, memorySchema);
          ids.push(m.id);
        }
        vars.ids = ids;
      },
    },
    {
      name: "page 0 envelope is internally consistent",
      run: async ({ client, vars }) => {
        const call = await listMemories(client, 0, 3);
        const list = expectShape(call, 200, memoryListSchema);
        assertEq(list.page, 0, "page echo");
        assertEq(list.size, 3, "size echo");
        assert(list.memories.length <= 3, "page returned more rows than size");
        assert(
          list.totalElements >= 6,
          `totalElements below seeded count: got ${list.totalElements}, expected ≥ 6 (size=${list.size} totalPages=${list.totalPages})`,
        );
        assertEq(
          list.totalPages,
          Math.ceil(list.totalElements / list.size),
          "totalPages math",
        );
        assertEq(list.hasPrevious, false, "page 0 hasPrevious");
        assertEq(
          list.hasNext,
          list.totalElements > list.size,
          "hasNext flag",
        );
        vars.page0 = list.memories.map((m) => m.id);
      },
    },
    {
      name: "page 1 has no rows in common with page 0",
      run: async ({ client, vars }) => {
        const call = await listMemories(client, 1, 3);
        const list = expectShape(call, 200, memoryListSchema);
        assertEq(list.page, 1, "page echo");
        assertEq(list.hasPrevious, true, "page 1 hasPrevious");
        const page0 = new Set(vars.page0 as string[]);
        const overlap = list.memories.filter((m) => page0.has(m.id));
        assertEq(overlap.length, 0, "rows duplicated across pages");
      },
    },
    {
      name: "cleanup: delete seeded memories",
      run: async ({ client, vars }) => {
        for (const id of vars.ids as string[]) {
          await tryDeleteMemory(client, id);
        }
      },
    },
  ],
};

// --- Scenario 4: category & importance filters ---

const filters: Scenario = {
  id: "filters",
  title: "Category and importance filters",
  description:
    "Creates a BUG/CRITICAL memory and checks both filter endpoints " +
    "return it AND return only rows matching the requested facet.",
  steps: [
    {
      name: "create a BUG / CRITICAL memory",
      run: async ({ client, vars }) => {
        const tag = marker();
        const call = await createMemory(client, {
          content: `[bs-explorer] filter probe ${tag}`,
          category: "BUG",
          importance: "CRITICAL",
          tags: ["bs-explorer", tag],
        });
        const m = expectShape(call, 201, memorySchema);
        vars.id = m.id;
      },
    },
    {
      name: "by-category/BUG returns the memory and only BUG rows",
      run: async ({ client, vars }) => {
        const call = await memoriesByCategory(client, "BUG");
        assertStatus(call, 200);
        const rows = expectShape(call, 200, memorySchema.array());
        assert(
          rows.some((m) => m.id === vars.id),
          "by-category did not return the seeded memory",
        );
        const wrong = rows.filter((m) => m.category && m.category !== "BUG");
        assertEq(wrong.length, 0, "by-category leaked non-BUG rows");
      },
    },
    {
      name: "by-importance/CRITICAL returns the memory and only CRITICAL rows",
      run: async ({ client, vars }) => {
        const call = await memoriesByImportance(client, "CRITICAL");
        const rows = expectShape(call, 200, memorySchema.array());
        assert(
          rows.some((m) => m.id === vars.id),
          "by-importance did not return the seeded memory",
        );
        const wrong = rows.filter(
          (m) => m.importance && m.importance !== "CRITICAL",
        );
        assertEq(wrong.length, 0, "by-importance leaked non-CRITICAL rows");
      },
    },
    {
      name: "cleanup: delete the probe memory",
      run: async ({ client, vars }) => {
        await tryDeleteMemory(client, vars.id as string | undefined);
      },
    },
  ],
};

// --- Scenario 5: feedback loop ---

const feedback: Scenario = {
  id: "feedback",
  title: "Feedback loop",
  description:
    "Records a helpful vote and checks the counter increments and the " +
    "feedback-weight endpoint agrees with it.",
  steps: [
    {
      name: "create a memory and capture its helpful count",
      run: async ({ client, vars }) => {
        const tag = marker();
        const call = await createMemory(client, {
          content: `[bs-explorer] feedback subject ${tag}`,
          category: "INSIGHT",
          tags: ["bs-explorer", tag],
        });
        const m = expectShape(call, 201, memorySchema);
        vars.id = m.id;
        vars.before = num(m.helpfulCount);
      },
    },
    {
      name: "POST a helpful=true vote",
      run: async ({ client, vars }) => {
        const call = await recordFeedback(client, vars.id as string, true);
        assertStatus(call, 200);
      },
    },
    {
      name: "helpful count increments by one",
      run: async ({ client, vars }) => {
        const call = await getMemory(client, vars.id as string);
        const m = expectShape(call, 200, memorySchema);
        assertEq(
          num(m.helpfulCount),
          num(vars.before) + 1,
          "helpfulCount after one vote",
        );
      },
    },
    {
      name: "feedback-weight endpoint agrees with the vote",
      run: async ({ client, vars }) => {
        const call = await feedbackWeight(client, vars.id as string);
        const w = expectShape(call, 200, feedbackWeightSchema);
        assertEq(w.helpfulCount, 1, "feedback-weight helpfulCount");
        assert(
          Number.isFinite(w.feedbackWeight),
          "feedbackWeight is not a finite number",
        );
      },
    },
    {
      name: "cleanup: delete the memory",
      run: async ({ client, vars }) => {
        await tryDeleteMemory(client, vars.id as string | undefined);
      },
    },
  ],
};

// --- Scenario 6: relationships ---

const relationships: Scenario = {
  id: "relationships",
  title: "Relationship create and query",
  description:
    "Links two memories, verifies the link appears in the outgoing list, " +
    "then removes it.",
  steps: [
    {
      name: "create two memories to link",
      run: async ({ client, vars }) => {
        const tag = marker();
        // unique() per memory so they aren't collapsed by SimHash dedup
        // (otherwise A and B would be the same id and the link is a self-loop).
        const a = expectShape(
          await createMemory(client, {
            content: `[bs-explorer] relationship source ${tag} ${unique()}`,
            category: "KNOWLEDGE",
            tags: ["bs-explorer", tag],
          }),
          201,
          memorySchema,
        );
        const b = expectShape(
          await createMemory(client, {
            content: `[bs-explorer] relationship target ${tag} ${unique()}`,
            category: "KNOWLEDGE",
            tags: ["bs-explorer", tag],
          }),
          201,
          memorySchema,
        );
        vars.a = a.id;
        vars.b = b.id;
      },
    },
    {
      name: "create a RELATED_TO relationship A -> B",
      run: async ({ client, vars }) => {
        const call = await createRelationship(
          client,
          vars.a as string,
          vars.b as string,
          "RELATED_TO",
        );
        assertStatus(call, 201);
      },
    },
    {
      name: "outgoing relationships of A reference B",
      run: async ({ client, vars }) => {
        const call = await relationshipsFrom(client, vars.a as string);
        assertStatus(call, 200);
        assert(Array.isArray(call.data), "from-relationships is not an array");
        assert(
          JSON.stringify(call.data).includes(vars.b as string),
          "the A->B relationship is missing from A's outgoing list",
        );
      },
    },
    {
      name: "related-memories endpoint responds for A",
      run: async ({ client, vars }) => {
        const call = await relatedMemories(client, vars.a as string);
        assertStatus(call, 200);
      },
    },
    {
      name: "the relationship can be deleted",
      run: async ({ client, vars }) => {
        const call = await deleteRelationshipBetween(
          client,
          vars.a as string,
          vars.b as string,
        );
        assertStatus(call, 200);
      },
    },
    {
      name: "cleanup: delete both memories",
      run: async ({ client, vars }) => {
        await tryDeleteMemory(client, vars.a as string | undefined);
        await tryDeleteMemory(client, vars.b as string | undefined);
      },
    },
  ],
};

// --- Scenario 7: pluggable store parity ---

const store: Scenario = {
  id: "store",
  title: "Pluggable store (/v1/store/memories)",
  description:
    "Exercises the MemoryStore-backed CRUD surface: create, get, search " +
    "and delete, including the 404 after delete.",
  steps: [
    {
      name: "store create returns 201 with an id",
      run: async ({ client, vars }) => {
        const tag = marker();
        vars.tag = tag;
        const call = await storeCreate(client, {
          content: `[bs-explorer] store entry ${tag}`,
          summary: "store scenario seed",
          category: "INSIGHT",
        });
        const m = expectShape(call, 201, memorySchema);
        vars.id = m.id;
      },
    },
    {
      name: "store get round-trips the content",
      run: async ({ client, vars }) => {
        const call = await storeGet(client, vars.id as string);
        const m = expectShape(call, 200, memorySchema);
        assert(
          m.content.includes(vars.tag as string),
          `store get lost the marker: "${m.content}"`,
        );
      },
    },
    {
      name: "store search returns a consistent envelope",
      run: async ({ client, vars }) => {
        const call = await storeSearch(client, vars.tag as string, 10);
        assertStatus(call, 200);
        const data = call.data as { results?: unknown[]; total?: number };
        assert(Array.isArray(data.results), "store search has no results[]");
        assertEq(
          data.total,
          data.results.length,
          "store search total vs results.length",
        );
      },
    },
    {
      name: "store delete returns 204",
      run: async ({ client, vars }) => {
        const call = await storeDelete(client, vars.id as string);
        assertStatus(call, 204);
      },
    },
    {
      name: "the deleted store memory returns 404",
      run: async ({ client, vars }) => {
        const call = await storeGet(client, vars.id as string);
        assertStatus(call, 404);
      },
    },
  ],
};

// --- Scenario: SimHash deduplication (documents the dedup contract) ---

const dedup: Scenario = {
  id: "dedup",
  title: "SimHash deduplication",
  description:
    "POSTing a near-duplicate memory (Hamming distance ≤ 3) returns the " +
    "existing memory's id with 201 — no second row is created. A textually " +
    "distant POST does create a new row. This documents intentional service " +
    "behavior consumers must know about.",
  steps: [
    {
      name: "create memory A",
      run: async ({ client, vars }) => {
        const tag = marker();
        const blob = unique();
        vars.tag = tag;
        vars.blob = blob;
        const call = await createMemory(client, {
          content: `[bs-explorer] dedup subject ${tag} ${blob}`,
          category: "INSIGHT",
        });
        const m = expectShape(call, 201, memorySchema);
        vars.idA = m.id;
      },
    },
    {
      name: "identical POST returns A's id (dedup, not a new row)",
      run: async ({ client, vars }) => {
        // Identical content → SimHash Hamming distance 0 → always dedups.
        // (Whether a NEAR-duplicate dedups depends on SimHash weight on
        // each differing token; the threshold of 3 is sensitive to short
        // high-entropy texts. The contract this step pins is the simpler,
        // unambiguous one: identical content returns the existing id.)
        const call = await createMemory(client, {
          content: `[bs-explorer] dedup subject ${vars.tag} ${vars.blob}`,
          category: "INSIGHT",
        });
        const m = expectShape(call, 201, memorySchema);
        assertEq(
          m.id,
          vars.idA,
          "second POST id (should match A's; SimHash dedup is the documented behavior)",
        );
      },
    },
    {
      name: "a textually distant POST DOES create a separate memory",
      run: async ({ client, vars }) => {
        const call = await createMemory(client, {
          content: `[bs-explorer] dedup unrelated topic ${marker()} ${unique()}`,
          category: "INSIGHT",
        });
        const m = expectShape(call, 201, memorySchema);
        assert(
          m.id !== vars.idA,
          "distant POST collapsed onto A — dedup threshold is too wide",
        );
        vars.idC = m.id;
      },
    },
    {
      name: "cleanup: delete A and the distant memory",
      run: async ({ client, vars }) => {
        await tryDeleteMemory(client, vars.idA as string | undefined);
        await tryDeleteMemory(client, vars.idC as string | undefined);
      },
    },
  ],
};

export const memoryScenarios: Scenario[] = [
  lifecycle,
  dedup,
  search,
  pagination,
  filters,
  feedback,
  relationships,
  store,
];
