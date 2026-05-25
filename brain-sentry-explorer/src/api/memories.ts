// Typed wrappers for the core memory endpoints. These are the functions
// the validation scenarios call — one per backend route under
// /v1/memories, /v1/relationships and /v1/store/memories.
//
// Each returns the raw ApiCall so scenarios can assert on status/latency
// as well as the decoded body.

import type { ApiCall, BrainSentryClient } from "./client.js";
import type {
  CreateMemoryRequest,
  Memory,
  MemoryList,
  SearchRequest,
  SearchResponse,
  UpdateMemoryRequest,
} from "./types.js";

// --- /v1/memories ---

export const createMemory = (c: BrainSentryClient, body: CreateMemoryRequest) =>
  c.request<Memory>("POST", "/v1/memories", { body });

export const getMemory = (c: BrainSentryClient, id: string) =>
  c.request<Memory>("GET", `/v1/memories/${id}`);

export const listMemories = (c: BrainSentryClient, page = 0, size = 20) =>
  c.request<MemoryList>("GET", "/v1/memories", { query: { page, size } });

export const updateMemory = (
  c: BrainSentryClient,
  id: string,
  body: UpdateMemoryRequest,
) => c.request<Memory>("PUT", `/v1/memories/${id}`, { body });

export const deleteMemory = (c: BrainSentryClient, id: string) =>
  c.request<{ message: string }>("DELETE", `/v1/memories/${id}`);

export const searchMemories = (c: BrainSentryClient, body: SearchRequest) =>
  c.request<SearchResponse>("POST", "/v1/memories/search", { body });

export const memoriesByCategory = (c: BrainSentryClient, category: string) =>
  c.request<Memory[]>("GET", `/v1/memories/by-category/${category}`);

export const memoriesByImportance = (c: BrainSentryClient, level: string) =>
  c.request<Memory[]>("GET", `/v1/memories/by-importance/${level}`);

export const memoryVersions = (c: BrainSentryClient, id: string) =>
  c.request<unknown[]>("GET", `/v1/memories/${id}/versions`);

export const recordFeedback = (
  c: BrainSentryClient,
  id: string,
  helpful: boolean,
) => c.request("POST", `/v1/memories/${id}/feedback`, { body: { helpful } });

export const feedbackWeight = (c: BrainSentryClient, id: string) =>
  c.request("GET", `/v1/memories/${id}/feedback-weight`);

export const flagMemory = (c: BrainSentryClient, id: string, reason: string) =>
  c.request("POST", `/v1/memories/${id}/flag`, { body: { reason } });

// --- /v1/relationships ---

export const createRelationship = (
  c: BrainSentryClient,
  fromMemoryId: string,
  toMemoryId: string,
  type: string,
) =>
  c.request("POST", "/v1/relationships", {
    body: { fromMemoryId, toMemoryId, type },
  });

export const relationshipsFrom = (c: BrainSentryClient, memoryId: string) =>
  c.request<unknown[]>("GET", `/v1/relationships/from/${memoryId}`);

export const relatedMemories = (c: BrainSentryClient, memoryId: string) =>
  c.request("GET", `/v1/relationships/${memoryId}/related`);

// The backend route accepts `from` and `to` query params (matching the
// GetBetween handler on the same path). The earlier `memoryId1`/`memoryId2`
// names looked plausible but produce a silent 400.
export const deleteRelationshipBetween = (
  c: BrainSentryClient,
  fromMemoryId: string,
  toMemoryId: string,
) =>
  c.request("DELETE", "/v1/relationships/between", {
    query: { from: fromMemoryId, to: toMemoryId },
  });

// --- /v1/store/memories (pluggable MemoryStore surface) ---

export const storeCreate = (c: BrainSentryClient, body: CreateMemoryRequest) =>
  c.request<Memory>("POST", "/v1/store/memories", { body });

export const storeGet = (c: BrainSentryClient, id: string) =>
  c.request<Memory>("GET", `/v1/store/memories/${id}`);

export const storeSearch = (c: BrainSentryClient, q: string, limit = 10) =>
  c.request<{ results: Memory[]; total: number }>(
    "GET",
    "/v1/store/memories/search",
    { query: { q, limit } },
  );

export const storeDelete = (c: BrainSentryClient, id: string) =>
  c.request("DELETE", `/v1/store/memories/${id}`);

// Generic helper: best-effort cleanup that ignores failures.
export async function tryDeleteMemory(
  c: BrainSentryClient,
  id: string | undefined,
): Promise<ApiCall | undefined> {
  if (!id) return undefined;
  return deleteMemory(c, id);
}
