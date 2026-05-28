// Endpoint catalog for the interactive explorer. Each entry is a
// ready-to-fire example call: a sample body, sample query and a path
// built from the explorer's context (IDs captured from earlier calls).
//
// This is the "how to use the API" half of the app — browsing the
// catalog and firing entries shows a working request/response for every
// core memory endpoint, with IDs chained automatically.

import type { HttpMethod } from "./api/client.js";

/** Mutable scratchpad: IDs captured from responses, keyed by name. */
export type ExplorerCtx = Record<string, string | undefined>;

export interface BuiltRequest {
  path: string;
  body?: unknown;
  query?: Record<string, string | number>;
}

export interface CatalogEndpoint {
  id: string;
  group: string;
  method: HttpMethod;
  /** Display label, e.g. "POST /v1/memories". */
  label: string;
  summary: string;
  /** Context keys that must be set before this endpoint can fire. */
  needs?: string[];
  /** Builds the concrete request from the current context. */
  build: (ctx: ExplorerCtx) => BuiltRequest;
  /** Captures IDs from a successful response into the context. */
  capture?: (data: unknown, ctx: ExplorerCtx) => void;
}

// Helper: pull a string field off an unknown response object.
function field(data: unknown, key: string): string | undefined {
  if (data && typeof data === "object") {
    const v = (data as Record<string, unknown>)[key];
    if (typeof v === "string") return v;
  }
  return undefined;
}

const sampleMemory = (label: string) => ({
  content: `[bs-explorer] ${label} created from the catalog`,
  summary: `${label} sample`,
  category: "KNOWLEDGE",
  importance: "IMPORTANT",
  tags: ["bs-explorer", "catalog"],
});

export const CATALOG: CatalogEndpoint[] = [
  // --- Memory CRUD ---
  {
    id: "mem-create",
    group: "Memory CRUD",
    method: "POST",
    label: "POST /v1/memories",
    summary: "Create a memory. Captures its id as {memoryId}.",
    build: () => ({ path: "/v1/memories", body: sampleMemory("primary memory") }),
    capture: (data, ctx) => {
      ctx.memoryId = field(data, "id") ?? ctx.memoryId;
    },
  },
  {
    id: "mem-create-b",
    group: "Memory CRUD",
    method: "POST",
    label: "POST /v1/memories (link target)",
    summary: "Create a second memory. Captures its id as {memoryIdB}.",
    build: () => ({
      path: "/v1/memories",
      body: sampleMemory("link target memory"),
    }),
    capture: (data, ctx) => {
      ctx.memoryIdB = field(data, "id") ?? ctx.memoryIdB;
    },
  },
  {
    id: "mem-list",
    group: "Memory CRUD",
    method: "GET",
    label: "GET /v1/memories",
    summary: "List memories, first page.",
    build: () => ({ path: "/v1/memories", query: { page: 0, size: 10 } }),
  },
  {
    id: "mem-get",
    group: "Memory CRUD",
    method: "GET",
    label: "GET /v1/memories/{id}",
    summary: "Fetch the memory captured as {memoryId}.",
    needs: ["memoryId"],
    build: (ctx) => ({ path: `/v1/memories/${ctx.memoryId}` }),
  },
  {
    id: "mem-update",
    group: "Memory CRUD",
    method: "PUT",
    label: "PUT /v1/memories/{id}",
    summary: "Update {memoryId}'s content (bumps its version).",
    needs: ["memoryId"],
    build: (ctx) => ({
      path: `/v1/memories/${ctx.memoryId}`,
      body: {
        content: "[bs-explorer] updated from the catalog explorer",
        changeReason: "catalog explorer edit",
      },
    }),
  },
  {
    id: "mem-delete",
    group: "Memory CRUD",
    method: "DELETE",
    label: "DELETE /v1/memories/{id}",
    summary: "Delete {memoryId}.",
    needs: ["memoryId"],
    build: (ctx) => ({ path: `/v1/memories/${ctx.memoryId}` }),
  },

  // --- Search & filter ---
  {
    id: "mem-search",
    group: "Search & filter",
    method: "POST",
    label: "POST /v1/memories/search",
    summary: "Semantic + full-text search.",
    build: () => ({
      path: "/v1/memories/search",
      body: { query: "bs-explorer", limit: 5, includeRelated: false },
    }),
  },
  {
    id: "mem-by-category",
    group: "Search & filter",
    method: "GET",
    label: "GET /v1/memories/by-category/{category}",
    summary: "All memories in the KNOWLEDGE category.",
    build: () => ({ path: "/v1/memories/by-category/KNOWLEDGE" }),
  },
  {
    id: "mem-by-importance",
    group: "Search & filter",
    method: "GET",
    label: "GET /v1/memories/by-importance/{importance}",
    summary: "All memories at IMPORTANT importance.",
    build: () => ({ path: "/v1/memories/by-importance/IMPORTANT" }),
  },

  // --- Feedback & versions ---
  {
    id: "mem-versions",
    group: "Feedback & versions",
    method: "GET",
    label: "GET /v1/memories/{id}/versions",
    summary: "Version history of {memoryId}.",
    needs: ["memoryId"],
    build: (ctx) => ({ path: `/v1/memories/${ctx.memoryId}/versions` }),
  },
  {
    id: "mem-feedback",
    group: "Feedback & versions",
    method: "POST",
    label: "POST /v1/memories/{id}/feedback",
    summary: "Record a helpful=true vote on {memoryId}.",
    needs: ["memoryId"],
    build: (ctx) => ({
      path: `/v1/memories/${ctx.memoryId}/feedback`,
      body: { helpful: true },
    }),
  },
  {
    id: "mem-feedback-weight",
    group: "Feedback & versions",
    method: "GET",
    label: "GET /v1/memories/{id}/feedback-weight",
    summary: "Current learned feedback weight of {memoryId}.",
    needs: ["memoryId"],
    build: (ctx) => ({ path: `/v1/memories/${ctx.memoryId}/feedback-weight` }),
  },
  {
    id: "mem-flag",
    group: "Feedback & versions",
    method: "POST",
    label: "POST /v1/memories/{id}/flag",
    summary: "Flag {memoryId} as incorrect, queuing it for review.",
    needs: ["memoryId"],
    build: (ctx) => ({
      path: `/v1/memories/${ctx.memoryId}/flag`,
      body: { reason: "bs-explorer catalog flag demo" },
    }),
  },

  // --- Relationships ---
  {
    id: "rel-create",
    group: "Relationships",
    method: "POST",
    label: "POST /v1/relationships",
    summary: "Link {memoryId} -> {memoryIdB} as RELATED_TO.",
    needs: ["memoryId", "memoryIdB"],
    build: (ctx) => ({
      path: "/v1/relationships",
      body: {
        fromMemoryId: ctx.memoryId,
        toMemoryId: ctx.memoryIdB,
        type: "RELATED_TO",
      },
    }),
  },
  {
    id: "rel-from",
    group: "Relationships",
    method: "GET",
    label: "GET /v1/relationships/from/{memoryId}",
    summary: "Outgoing relationships of {memoryId}.",
    needs: ["memoryId"],
    build: (ctx) => ({ path: `/v1/relationships/from/${ctx.memoryId}` }),
  },
  {
    id: "rel-related",
    group: "Relationships",
    method: "GET",
    label: "GET /v1/relationships/{memoryId}/related",
    summary: "Related memories of {memoryId}.",
    needs: ["memoryId"],
    build: (ctx) => ({ path: `/v1/relationships/${ctx.memoryId}/related` }),
  },
  {
    id: "rel-delete",
    group: "Relationships",
    method: "DELETE",
    label: "DELETE /v1/relationships/between",
    summary: "Remove the link between {memoryId} and {memoryIdB}.",
    needs: ["memoryId", "memoryIdB"],
    build: (ctx) => ({
      path: "/v1/relationships/between",
      query: { from: ctx.memoryId ?? "", to: ctx.memoryIdB ?? "" },
    }),
  },

  // --- Pluggable store ---
  {
    id: "store-create",
    group: "Pluggable store",
    method: "POST",
    label: "POST /v1/store/memories",
    summary: "Create via the MemoryStore surface. Captures {storeId}.",
    build: () => ({
      path: "/v1/store/memories",
      body: {
        content: "[bs-explorer] store-backed memory from the catalog",
        summary: "store sample",
        category: "INSIGHT",
      },
    }),
    capture: (data, ctx) => {
      ctx.storeId = field(data, "id") ?? ctx.storeId;
    },
  },
  {
    id: "store-list",
    group: "Pluggable store",
    method: "GET",
    label: "GET /v1/store/memories",
    summary: "List store-backed memories.",
    build: () => ({ path: "/v1/store/memories", query: { limit: 10 } }),
  },
  {
    id: "store-get",
    group: "Pluggable store",
    method: "GET",
    label: "GET /v1/store/memories/{id}",
    summary: "Fetch the store memory captured as {storeId}.",
    needs: ["storeId"],
    build: (ctx) => ({ path: `/v1/store/memories/${ctx.storeId}` }),
  },
  {
    id: "store-search",
    group: "Pluggable store",
    method: "GET",
    label: "GET /v1/store/memories/search",
    summary: "Search store-backed memories.",
    build: () => ({
      path: "/v1/store/memories/search",
      query: { q: "bs-explorer", limit: 10 },
    }),
  },
  {
    id: "store-delete",
    group: "Pluggable store",
    method: "DELETE",
    label: "DELETE /v1/store/memories/{id}",
    summary: "Delete the store memory {storeId}.",
    needs: ["storeId"],
    build: (ctx) => ({ path: `/v1/store/memories/${ctx.storeId}` }),
  },
];

/** Catalog grouped in display order. */
export function catalogGroups(): { group: string; endpoints: CatalogEndpoint[] }[] {
  const order: string[] = [];
  const byGroup = new Map<string, CatalogEndpoint[]>();
  for (const ep of CATALOG) {
    if (!byGroup.has(ep.group)) {
      byGroup.set(ep.group, []);
      order.push(ep.group);
    }
    byGroup.get(ep.group)!.push(ep);
  }
  return order.map((group) => ({ group, endpoints: byGroup.get(group)! }));
}
