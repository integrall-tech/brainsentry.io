-- Receipts for data-subject erasure and retention runs (RFC-014 §10).
--
-- A deletion you cannot prove is a deletion you will be asked to prove again.
-- The receipt is the artefact that answers "was this person's data removed,
-- when, and how much of it" AFTER the data itself is gone — which is exactly
-- when nothing else can answer it.
--
-- It deliberately holds NO subject content: only the scope expression that
-- was executed, per-surface counts, and timing. Keeping the erased text here
-- to "document" the erasure would defeat it.

CREATE TABLE IF NOT EXISTS erasure_receipts (
    id            VARCHAR(100) PRIMARY KEY,
    tenant_id     VARCHAR(100) NOT NULL,
    -- "erasure" (data-subject request) or "retention" (policy sweep).
    kind          VARCHAR(30)  NOT NULL,
    -- The scope that was resolved, as given: tag / ids / sourceReference.
    -- A tag like "cliente:acme" is an identifier, not content.
    scope         JSONB        NOT NULL DEFAULT '{}',
    -- Rows affected per surface: {"memories": 12, "memory_versions": 30, ...}
    counts        JSONB        NOT NULL DEFAULT '{}',
    -- Free-text justification supplied by the caller (ticket id, legal basis).
    reason        TEXT,
    requested_by  VARCHAR(100),
    -- False for a dry run: the plan is recorded too, so an operator can show
    -- what a destructive run WOULD have done before authorising it.
    executed      BOOLEAN      NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_erasure_receipts_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_erasure_receipts_tenant
    ON erasure_receipts (tenant_id, created_at DESC);
