// Validation scenarios for interactive conflict resolution
// (POST /v1/conflicts/resolve) — the human-in-the-loop counterpart to
// the automatic detection endpoints. Proves supersede actually marks the
// loser stale (trust capped) and promotes the winner, and that dismiss
// leaves both untouched.

import {
  createMemory,
  getMemory,
  getTrust,
  resolveConflict,
  tryDeleteMemory,
} from "../api/memories.js";
import { memorySchema, trustReportSchema } from "../api/types.js";
import { assert, assertEq, assertStatus, expectShape } from "./assert.js";
import type { Scenario } from "./runner.js";

function marker(): string {
  return `bsx${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

const supersede: Scenario = {
  id: "conflict-resolve-supersede",
  title: "Conflict resolve — supersede",
  description:
    "Resolve a conflicting pair by superseding the loser: the loser must " +
    "become superseded (trust capped at 0.30) and the winner promoted to " +
    "CORRECTED provenance.",
  steps: [
    {
      name: "create two conflicting memories",
      run: async ({ client, vars }) => {
        const tag = marker();
        const a = expectShape(
          await createMemory(client, {
            content: `[bs-explorer] conflict ${tag}: the deploy uses Docker Swarm`,
            category: "KNOWLEDGE",
            provenance: "OBSERVED",
          }),
          201,
          memorySchema,
        );
        const b = expectShape(
          await createMemory(client, {
            content: `[bs-explorer] conflict ${tag}: the deploy uses Kubernetes`,
            category: "KNOWLEDGE",
            provenance: "INFERRED",
          }),
          201,
          memorySchema,
        );
        vars.winner = a.id;
        vars.loser = b.id;
      },
    },
    {
      name: "resolve(supersede) returns resolved=true",
      run: async ({ client, vars }) => {
        const call = await resolveConflict(
          client,
          vars.winner as string,
          vars.loser as string,
          "supersede",
        );
        assertStatus(call, 200);
        assertEq(
          (call.data as { resolved?: boolean }).resolved,
          true,
          "resolve should report resolved=true",
        );
      },
    },
    {
      name: "loser is superseded and its trust is capped at 0.30",
      run: async ({ client, vars }) => {
        const m = expectShape(
          await getMemory(client, vars.loser as string),
          200,
          memorySchema,
        );
        assert(
          !!(m as { supersededBy?: string }).supersededBy,
          "loser should carry supersededBy",
        );
        const t = expectShape(
          await getTrust(client, vars.loser as string),
          200,
          trustReportSchema,
        );
        assert(t.score <= 0.30, `loser trust should be <= 0.30; got ${t.score}`);
      },
    },
    {
      name: "winner is promoted to CORRECTED provenance",
      run: async ({ client, vars }) => {
        const m = expectShape(
          await getMemory(client, vars.winner as string),
          200,
          memorySchema,
        );
        assertEq(
          (m as { provenance?: string }).provenance,
          "CORRECTED",
          "winner should be promoted to CORRECTED after resolving the conflict",
        );
      },
    },
    {
      name: "cleanup",
      run: async ({ client, vars }) => {
        await tryDeleteMemory(client, vars.winner as string | undefined);
        await tryDeleteMemory(client, vars.loser as string | undefined);
      },
    },
  ],
};

const dismiss: Scenario = {
  id: "conflict-resolve-dismiss",
  title: "Conflict resolve — dismiss",
  description:
    "Dismissing a pair (not actually a conflict) leaves both memories " +
    "untouched — neither is superseded.",
  steps: [
    {
      name: "create two memories",
      run: async ({ client, vars }) => {
        const tag = marker();
        const a = expectShape(
          await createMemory(client, {
            content: `[bs-explorer] dismiss ${tag} alpha`,
            category: "CONTEXT",
          }),
          201,
          memorySchema,
        );
        const b = expectShape(
          await createMemory(client, {
            content: `[bs-explorer] dismiss ${tag} beta`,
            category: "CONTEXT",
          }),
          201,
          memorySchema,
        );
        vars.a = a.id;
        vars.b = b.id;
      },
    },
    {
      name: "resolve(dismiss) keeps both untouched",
      run: async ({ client, vars }) => {
        const call = await resolveConflict(
          client,
          vars.a as string,
          vars.b as string,
          "dismiss",
        );
        assertStatus(call, 200);
        for (const id of [vars.a, vars.b] as string[]) {
          const m = expectShape(await getMemory(client, id), 200, memorySchema);
          assert(
            !(m as { supersededBy?: string }).supersededBy,
            `dismiss must not supersede ${id}`,
          );
        }
      },
    },
    {
      name: "invalid action is rejected with 400",
      run: async ({ client, vars }) => {
        const call = await resolveConflict(
          client,
          vars.a as string,
          vars.b as string,
          "merge" as "dismiss",
        );
        assertStatus(call, 400);
      },
    },
    {
      name: "cleanup",
      run: async ({ client, vars }) => {
        await tryDeleteMemory(client, vars.a as string | undefined);
        await tryDeleteMemory(client, vars.b as string | undefined);
      },
    },
  ],
};

export const conflictScenarios: Scenario[] = [supersede, dismiss];
