// Capability detection. Runs once at the start of every validation run so
// scenarios that depend on optional backend features (FalkorDB, pgvector,
// LLM provider, real embeddings) can be skipped honestly instead of
// failing — the report tells you "28 passed, 4 skipped (no llm key)".
//
// Each capability is probed against the running backend. Env overrides
// (BS_CAP_LLM=on|off|auto and friends) let you force a value when probing
// is unreliable.

import type { BrainSentryClient } from "./api/client.js";

export interface Capabilities {
  /** Postgres reachable (the backend itself can't boot without this). */
  postgres: boolean;
  /** FalkorDB reachable — required for /v1/graph/* and entity-graph. */
  falkordb: boolean;
  /** Migration 8 applied — required for /v1/decisions, policies, events. */
  pgvector: boolean;
  /** At least one LLM provider configured + reachable. */
  llm: boolean;
  /** Real embedding provider keyed (vs. the hashEmbed fallback). */
  embeddingReal: boolean;
}

export type Capability = keyof Capabilities;

const envOverride = (name: string): boolean | undefined => {
  const v = process.env[name]?.toLowerCase();
  if (v === "on" || v === "true" || v === "1") return true;
  if (v === "off" || v === "false" || v === "0") return false;
  return undefined; // "auto" or unset → probe
};

export async function detectCapabilities(
  client: BrainSentryClient,
): Promise<Capabilities> {
  // --- /v1/diagnostics: postgres + falkordb in one round-trip ---
  const diag = await client.request<{
    checks?: { name?: string; status?: string }[];
  }>("GET", "/v1/diagnostics");
  const checks = diag.data?.checks ?? [];
  const probed = {
    postgres:
      checks.find((c) => c.name === "postgres")?.status === "ok",
    falkordb:
      checks.find((c) => c.name === "falkordb")?.status === "ok",
  };

  // --- /v1/models/doctor: report `ok: true` when ≥1 prober succeeds ---
  const md = await client.request<{ ok?: boolean }>("GET", "/v1/models/doctor");
  const llmProbed = md.ok && md.data?.ok === true;

  // --- /v1/decisions/: 500 with "decisions does not exist" means
  //     migration 8 hasn't run (no pgvector). Anything else means it has.
  const decProbe = await client.request<{ message?: string }>(
    "GET",
    "/v1/decisions/?limit=1",
  );
  const pgvectorMissing =
    decProbe.status === 500 &&
    (decProbe.data?.message ?? "").includes('"decisions" does not exist');
  const pgvectorProbed = !pgvectorMissing;

  // --- Real-vs-hash embeddings: not reliably introspectable from outside,
  //     so we default to off unless explicitly opted-in via env. ---
  const embeddingProbed = false;

  return {
    postgres: envOverride("BS_CAP_POSTGRES") ?? probed.postgres,
    falkordb: envOverride("BS_CAP_FALKORDB") ?? probed.falkordb,
    pgvector: envOverride("BS_CAP_PGVECTOR") ?? pgvectorProbed,
    llm: envOverride("BS_CAP_LLM") ?? llmProbed,
    embeddingReal: envOverride("BS_CAP_EMBEDDING_REAL") ?? embeddingProbed,
  };
}

/** Capabilities required-but-missing for a given scenario. */
export function missingCapabilities(
  available: Capabilities,
  required: readonly Capability[] | undefined,
): Capability[] {
  if (!required || required.length === 0) return [];
  return required.filter((c) => !available[c]);
}

/** Human-readable line for the run header. */
export function formatCapabilities(c: Capabilities): string {
  const mark = (b: boolean) => (b ? "✓" : "✗");
  return (
    `postgres ${mark(c.postgres)}  falkordb ${mark(c.falkordb)}  ` +
    `pgvector ${mark(c.pgvector)}  llm ${mark(c.llm)}  ` +
    `embedding-real ${mark(c.embeddingReal)}`
  );
}
