-- Service API keys: long-lived credentials scoped to exactly one tenant.
--
-- Why this exists (RFC-014 §8.1): the only credential today is a user JWT
-- with refresh. A service consuming the API is not a person — storing a
-- user's password to renew a token is worse than storing a key, and the
-- refresh adds a failure mode (token expiring mid-batch) that need not
-- exist. More importantly, it removes the cross-tenant hazard: a key
-- cannot reach another tenant, so the worst defect of the integration
-- (reading or writing another customer's memory) stops depending on code
-- discipline and becomes impossible by credential.
--
-- key_hash is SHA-256 of the secret, hex-encoded. Not bcrypt: the secret is
-- 256 bits of CSPRNG output, not a human password, so there is no
-- dictionary to defend against — and bcrypt would add ~100ms to every
-- authenticated request on the recall hot path.
--
-- key_prefix is the first characters of the secret, stored in clear. It is
-- how a request finds its candidate row without scanning: the lookup is by
-- prefix, and the decision is the constant-time hash comparison.

CREATE TABLE IF NOT EXISTS api_keys (
    id            VARCHAR(100) PRIMARY KEY,
    tenant_id     VARCHAR(100) NOT NULL,
    name          VARCHAR(255) NOT NULL,
    -- Clear-text lookup handle (e.g. "bs_a1b2c3d4"). Never the secret.
    key_prefix    VARCHAR(32)  NOT NULL,
    -- SHA-256 hex of the full secret. The secret itself is shown once, at
    -- creation, and is not recoverable from here.
    key_hash      VARCHAR(64)  NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by    VARCHAR(100),
    -- Throttled write (see the repository): updating this on every request
    -- would turn a read path into a write path.
    last_used_at  TIMESTAMPTZ,
    -- Revocation is a tombstone, not a delete: an audit trail of which key
    -- acted is worthless if the key row can vanish.
    revoked_at    TIMESTAMPTZ,
    expires_at    TIMESTAMPTZ,

    CONSTRAINT fk_api_keys_tenant FOREIGN KEY (tenant_id)
        REFERENCES tenants (id) ON DELETE CASCADE
);

-- The authentication lookup: by prefix, only among live keys.
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix
    ON api_keys (key_prefix)
    WHERE revoked_at IS NULL;

-- Two different keys must never share a hash; also makes the hash lookup
-- exact rather than "first match wins".
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_hash
    ON api_keys (key_hash);

-- Listing a tenant's keys in the admin surface.
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant
    ON api_keys (tenant_id, created_at DESC);
