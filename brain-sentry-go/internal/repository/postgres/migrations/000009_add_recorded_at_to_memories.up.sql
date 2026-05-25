-- Add the recorded_at column the Go code already references.
--
-- memory.go's memoryColumns list, INSERT, scanMemory, and ORDER BY clauses
-- (see internal/repository/postgres/memory.go) all expect a recorded_at
-- column on memories. Earlier migrations never added it, so a fresh schema
-- breaks every INSERT INTO memories with SQLSTATE 42703.
--
-- Surfaced end-to-end by the brain-sentry-explorer validation suite
-- (memory CRUD lifecycle scenario, POST /v1/memories returning 500).

ALTER TABLE memories
    ADD COLUMN IF NOT EXISTS recorded_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Index on (tenant_id, recorded_at) backs the bi-temporal "as-of" query in
-- memory.go (`AND recorded_at <= $2 ... ORDER BY recorded_at DESC`).
CREATE INDEX IF NOT EXISTS idx_memories_recorded_at
    ON memories (tenant_id, recorded_at DESC)
    WHERE deleted_at IS NULL;
