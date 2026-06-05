// Validation scenarios for provenance + trust scoring (the memanto-
// inspired confidence layer). These assert the SECOND-order property:
// not just "the API stores provenance" but "the trust score actually
// reflects it" — explicit > inferred, rejected drives low, the embedded
// trust on GET matches the standalone /trust endpoint, etc.

import {
  createMemory,
  flagMemory,
  getMemory,
  getTrust,
  recordFeedback,
  tryDeleteMemory,
} from "../api/memories.js";
import { memorySchema, trustReportSchema } from "../api/types.js";
import { assert, assertEq, assertStatus, expectShape } from "./assert.js";
import type { Scenario } from "./runner.js";

function marker(): string {
  return `bsx${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

// --- Scenario 1: provenance round-trips + drives trust ---

const provenanceRoundTrip: Scenario = {
  id: "trust-provenance",
  title: "Provenance round-trip + trust ordering",
  description:
    "A memory created with provenance INFERRED must persist that value " +
    "and score lower trust than one created EXPLICIT — proving provenance " +
    "is both stored and wired into the trust score.",
  steps: [
    {
      name: "create an EXPLICIT memory",
      run: async ({ client, vars }) => {
        const tag = marker();
        vars.tag = tag;
        const call = await createMemory(client, {
          content: `[bs-explorer] explicit fact ${tag}: the deploy runs on swarm`,
          category: "KNOWLEDGE",
          provenance: "EXPLICIT",
        });
        const m = expectShape(call, 201, memorySchema);
        vars.explicitId = m.id;
        assertEq(
          (m as { provenance?: string }).provenance,
          "EXPLICIT",
          "create response should echo provenance EXPLICIT",
        );
      },
    },
    {
      name: "create an INFERRED memory",
      run: async ({ client, vars }) => {
        const call = await createMemory(client, {
          content: `[bs-explorer] inferred guess ${vars.tag}: probably uses swarm`,
          category: "KNOWLEDGE",
          provenance: "INFERRED",
        });
        const m = expectShape(call, 201, memorySchema);
        vars.inferredId = m.id;
        assertEq(
          (m as { provenance?: string }).provenance,
          "INFERRED",
          "create response should echo provenance INFERRED",
        );
      },
    },
    {
      name: "GET embeds a trust report on each memory",
      run: async ({ client, vars }) => {
        const call = await getMemory(client, vars.explicitId as string);
        assertStatus(call, 200);
        const trust = (call.data as { trust?: unknown }).trust;
        const t = trustReportSchema.parse(trust);
        assert(
          t.reasons.length > 0,
          "trust report must carry at least one reason",
        );
        vars.explicitTrust = t.score;
      },
    },
    {
      name: "EXPLICIT scores higher trust than INFERRED",
      run: async ({ client, vars }) => {
        const inf = await getTrust(client, vars.inferredId as string);
        const infTrust = expectShape(inf, 200, trustReportSchema);
        assert(
          (vars.explicitTrust as number) > infTrust.score,
          `explicit trust (${vars.explicitTrust}) should exceed inferred (${infTrust.score})`,
        );
      },
    },
    {
      name: "the standalone /trust endpoint matches the embedded report",
      run: async ({ client, vars }) => {
        const embedded = await getMemory(client, vars.explicitId as string);
        const standalone = await getTrust(client, vars.explicitId as string);
        const e = (embedded.data as { trust?: { score: number } }).trust;
        const s = expectShape(standalone, 200, trustReportSchema);
        assertEq(
          s.score,
          e?.score,
          "standalone /trust score must equal the embedded trust.score",
        );
      },
    },
    {
      name: "cleanup",
      run: async ({ client, vars }) => {
        await tryDeleteMemory(client, vars.explicitId as string | undefined);
        await tryDeleteMemory(client, vars.inferredId as string | undefined);
      },
    },
  ],
};

// --- Scenario 2: feedback + flag move the trust score the right way ---

const trustReactsToSignals: Scenario = {
  id: "trust-signals",
  title: "Trust score reacts to feedback and flagging",
  description:
    "Positive feedback should raise trust; flagging a memory as incorrect " +
    "should lower it. Proves the score is live, not a static field.",
  steps: [
    {
      name: "create a baseline OBSERVED memory and capture its trust",
      run: async ({ client, vars }) => {
        const tag = marker();
        const call = await createMemory(client, {
          content: `[bs-explorer] observed behavior ${tag}`,
          category: "CONTEXT",
          provenance: "OBSERVED",
        });
        const m = expectShape(call, 201, memorySchema);
        vars.id = m.id;
        const t = expectShape(
          await getTrust(client, m.id),
          200,
          trustReportSchema,
        );
        vars.baseline = t.score;
      },
    },
    {
      name: "two helpful votes raise the trust score",
      run: async ({ client, vars }) => {
        // >=2 votes are needed before feedback counts.
        await recordFeedback(client, vars.id as string, true);
        await recordFeedback(client, vars.id as string, true);
        const t = expectShape(
          await getTrust(client, vars.id as string),
          200,
          trustReportSchema,
        );
        assert(
          t.score > (vars.baseline as number),
          `trust after positive feedback (${t.score}) should exceed baseline (${vars.baseline})`,
        );
        vars.afterFeedback = t.score;
      },
    },
    {
      name: "flagging the memory drops its trust below baseline",
      run: async ({ client, vars }) => {
        const flag = await flagMemory(
          client,
          vars.id as string,
          "bs-explorer trust scenario: marking suspect",
        );
        assertStatus(flag, 200, 201, 202);
        const t = expectShape(
          await getTrust(client, vars.id as string),
          200,
          trustReportSchema,
        );
        assert(
          t.score < (vars.afterFeedback as number),
          `trust after flagging (${t.score}) should fall below the post-feedback score (${vars.afterFeedback})`,
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

// --- Scenario 3: default provenance + label bucketing ---

const trustDefaults: Scenario = {
  id: "trust-defaults",
  title: "Default provenance + label bucketing",
  description:
    "A POST without provenance defaults to EXPLICIT (direct statement), " +
    "and the trust report always carries a valid high/medium/low label.",
  steps: [
    {
      name: "create a memory WITHOUT provenance",
      run: async ({ client, vars }) => {
        const call = await createMemory(client, {
          content: `[bs-explorer] no provenance given ${marker()}`,
          category: "KNOWLEDGE",
        });
        const m = expectShape(call, 201, memorySchema);
        vars.id = m.id;
        assertEq(
          (m as { provenance?: string }).provenance,
          "EXPLICIT",
          "missing provenance should default to EXPLICIT on a direct POST",
        );
      },
    },
    {
      name: "trust report has a valid label and bounded score",
      run: async ({ client, vars }) => {
        const t = expectShape(
          await getTrust(client, vars.id as string),
          200,
          trustReportSchema,
        );
        assert(
          ["high", "medium", "low"].includes(t.label),
          `unexpected trust label: ${t.label}`,
        );
        assert(
          t.score >= 0 && t.score <= 1,
          `trust score out of [0,1]: ${t.score}`,
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

export const trustScenarios: Scenario[] = [
  provenanceRoundTrip,
  trustReactsToSignals,
  trustDefaults,
];
