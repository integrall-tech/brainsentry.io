-- Indexes for deterministic retrieval (RFC-014 fatia 1).
--
-- The audit job asks "give me the fact produced by event X" — exact lookup,
-- not similarity. Without these two indexes that question is a sequential
-- scan of every memory in the tenant, which is fine at pilot volume and
-- becomes the whole latency budget once it is not.

-- Exact lookup by the domain event that produced the memory
-- ("decisao:{id}", "cotacao:{id}", "resolucao:{id}", "sentimento:{conv}").
-- Tenant first: every query is tenant-scoped, so it is the selective prefix.
CREATE INDEX IF NOT EXISTS idx_memories_source_reference
    ON memories (tenant_id, source_reference)
    WHERE source_reference IS NOT NULL AND deleted_at IS NULL;

-- Containment lookups on metadata (WHERE metadata @> '{"k":"v"}').
--
-- jsonb_path_ops rather than the default jsonb_ops: it indexes only the
-- containment operator, which is the only one this filter uses, and produces
-- a markedly smaller index for it. The trade-off — no support for key-exists
-- (?) queries — costs us nothing here.
CREATE INDEX IF NOT EXISTS idx_memories_metadata
    ON memories USING GIN (metadata jsonb_path_ops)
    WHERE metadata IS NOT NULL AND deleted_at IS NULL;
