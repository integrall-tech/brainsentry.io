-- Add the provenance column backing domain.Provenance + Memory.TrustScore.
--
-- Provenance records HOW a memory came to be known (EXPLICIT / VALIDATED
-- / CORRECTED / OBSERVED / IMPORTED / INFERRED) and is the strongest
-- single input to the trust score. Nullable + empty-string default so
-- existing rows keep working (they score from the neutral 0.50 base).

ALTER TABLE memories
    ADD COLUMN IF NOT EXISTS provenance VARCHAR(40) NOT NULL DEFAULT '';

-- Partial index for "list low-trust / inferred memories" style queries,
-- which the trust surfaces will lean on. Only indexes rows that actually
-- carry a provenance, keeping it small.
CREATE INDEX IF NOT EXISTS idx_memories_provenance
    ON memories (tenant_id, provenance)
    WHERE provenance <> '' AND deleted_at IS NULL;
