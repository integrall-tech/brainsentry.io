// Validation scenarios for the bi-temporal incremental-sync endpoint
// GET /v1/memories/changed-since — the complement of as-of. Proves an
// agent can pull just what moved since its last sync, and that the
// parameter validation matches the documented contract.

import {
  changedSince,
  createMemory,
  tryDeleteMemory,
} from "../api/memories.js";
import { memorySchema } from "../api/types.js";
import { assert, assertStatus, expectShape } from "./assert.js";
import type { Scenario } from "./runner.js";

function marker(): string {
  return `bsx${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

const changedSinceScenario: Scenario = {
  id: "temporal-changed-since",
  title: "changed-since incremental sync",
  description:
    "Captures a watermark, creates a memory after it, and asserts " +
    "changed-since(watermark) returns the new memory while a future " +
    "watermark returns nothing. Validates the agent incremental-sync path.",
  steps: [
    {
      name: "rejects a missing 'since' param with 400",
      run: async ({ client }) => {
        const call = await client.request("GET", "/v1/memories/changed-since");
        assertStatus(call, 400);
      },
    },
    {
      name: "rejects a non-RFC3339 'since' with 400",
      run: async ({ client }) => {
        const call = await client.request(
          "GET",
          "/v1/memories/changed-since",
          { query: { since: "not-a-date" } },
        );
        assertStatus(call, 400);
      },
    },
    {
      name: "create a memory and remember the watermark just before it",
      run: async ({ client, vars }) => {
        // Watermark a few seconds in the past to avoid clock-skew races
        // between this client and the server.
        vars.watermark = new Date(Date.now() - 5000).toISOString();
        const tag = marker();
        vars.tag = tag;
        const call = await createMemory(client, {
          content: `[bs-explorer] changed-since subject ${tag}`,
          category: "CONTEXT",
        });
        const m = expectShape(call, 201, memorySchema);
        vars.id = m.id;
      },
    },
    {
      name: "changed-since(watermark) includes the new memory",
      run: async ({ client, vars }) => {
        const call = await changedSince(client, vars.watermark as string, 100);
        assertStatus(call, 200);
        const data = call.data as { count: number; memories: { id: string }[] };
        assert(Array.isArray(data.memories), "response has no memories[]");
        assert(
          data.memories.some((m) => m.id === vars.id),
          "changed-since since the watermark should include the just-created memory",
        );
      },
    },
    {
      name: "changed-since(future) returns nothing",
      run: async ({ client, vars }) => {
        const future = new Date(Date.now() + 3600_000).toISOString();
        const call = await changedSince(client, future, 100);
        assertStatus(call, 200);
        const data = call.data as { count: number; memories: { id: string }[] };
        assert(
          !data.memories.some((m) => m.id === vars.id),
          "a future watermark must not return the memory created in the past",
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

export const temporalScenarios: Scenario[] = [changedSinceScenario];
