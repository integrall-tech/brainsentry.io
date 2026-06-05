DROP INDEX IF EXISTS idx_memories_provenance;

ALTER TABLE memories DROP COLUMN IF EXISTS provenance;
