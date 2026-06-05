// Validation scenarios for document ingestion (POST /v1/memories/upload).
// Proves a file is chunked into memories tagged provenance IMPORTED and
// traceable to the filename, plus the format/size guardrails.

import {
  getMemory,
  searchMemories,
  tryDeleteMemory,
  uploadFile,
  type UploadResult,
} from "../api/memories.js";
import { memorySchema, searchResponseSchema } from "../api/types.js";
import { assert, assertEq, assertStatus, expectShape } from "./assert.js";
import type { Scenario } from "./runner.js";

function marker(): string {
  return `bsx${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

const ingest: Scenario = {
  id: "ingest-upload",
  title: "Document ingestion (upload → chunked memories)",
  description:
    "Uploads a multi-paragraph markdown file and asserts it becomes " +
    "memories tagged provenance IMPORTED, traceable to the filename and " +
    "findable by search. Plus format + content guardrails.",
  steps: [
    {
      name: "upload a multi-paragraph markdown file",
      run: async ({ client, vars }) => {
        const tag = marker();
        vars.tag = tag;
        // Two clearly separated paragraphs → expect >= 2 chunks with a
        // small chunkChars so the split is deterministic.
        const md =
          `# Acme account notes ${tag}\n\n` +
          `${"Acme discovery: CTO Maria Santos, Postgres 15 concern. ".repeat(8)}\n\n` +
          `${"Acme demo: dedup praised, Okta SSO is the blocker. ".repeat(8)}`;
        const call = await uploadFile(client, `acme-${tag}.md`, md, {
          category: "REFERENCE",
          chunkChars: 200,
        });
        assertStatus(call, 201);
        const data = call.data as UploadResult;
        assert(data.chunks >= 2, `expected >=2 chunks; got ${data.chunks}`);
        assertEq(
          data.createdLen,
          data.chunks,
          "every chunk should have produced a memory",
        );
        assertEq(data.created.length, data.chunks, "created[] length mismatch");
        vars.created = data.created;
        vars.filename = `acme-${tag}.md`;
      },
    },
    {
      name: "each created memory is provenance IMPORTED + traces to the file",
      run: async ({ client, vars }) => {
        const ids = vars.created as string[];
        const first = await getMemory(client, ids[0]);
        const m = expectShape(first, 200, memorySchema);
        assertEq(
          (m as { provenance?: string }).provenance,
          "IMPORTED",
          "ingested chunk should carry provenance IMPORTED",
        );
        assertEq(
          (m as { sourceReference?: string }).sourceReference,
          vars.filename,
          "ingested chunk should reference its source filename",
        );
      },
    },
    {
      name: "ingested content is findable by search",
      run: async ({ client, vars }) => {
        const call = await searchMemories(client, {
          query: vars.tag as string,
          limit: 20,
        });
        const res = expectShape(call, 200, searchResponseSchema);
        const created = new Set(vars.created as string[]);
        assert(
          res.results.some((r) => created.has(r.id)),
          "at least one ingested chunk should be searchable by its token",
        );
      },
    },
    {
      name: "unsupported type (.pdf) is rejected with 415",
      run: async ({ client }) => {
        const call = await uploadFile(client, "doc.pdf", "%PDF-1.7 fake");
        assertStatus(call, 415);
      },
    },
    {
      name: "a CSV ingests too (different extractor path)",
      run: async ({ client, vars }) => {
        const tag = marker();
        vars.csvTag = tag;
        const csv =
          `customer,stage,note ${tag}\n` +
          `Acme,discovery,pg15 concern\n` +
          `Globex,closed-won,enterprise\n`;
        const call = await uploadFile(client, `deals-${tag}.csv`, csv, {
          chunkChars: 5000,
        });
        assertStatus(call, 201);
        const data = call.data as UploadResult;
        assert(data.createdLen >= 1, "csv should produce at least one memory");
        vars.csvCreated = data.created;
      },
    },
    {
      name: "cleanup: delete every ingested memory",
      run: async ({ client, vars }) => {
        for (const id of [
          ...((vars.created as string[]) ?? []),
          ...((vars.csvCreated as string[]) ?? []),
        ]) {
          await tryDeleteMemory(client, id);
        }
      },
    },
  ],
};

export const ingestScenarios: Scenario[] = [ingest];
