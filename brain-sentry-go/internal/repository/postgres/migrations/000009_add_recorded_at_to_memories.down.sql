DROP INDEX IF EXISTS idx_memories_recorded_at;

ALTER TABLE memories DROP COLUMN IF EXISTS recorded_at;
