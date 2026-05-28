-- Brain Sentry initial schema

-- Tenants
CREATE TABLE IF NOT EXISTS tenants (
    id VARCHAR(100) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    active BOOLEAN NOT NULL DEFAULT true,
    max_memories INTEGER NOT NULL DEFAULT 0,
    max_users INTEGER NOT NULL DEFAULT 0,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenants_slug ON tenants(slug);
CREATE INDEX IF NOT EXISTS idx_tenants_active ON tenants(active);

-- Users
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(100) PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255),
    password_hash VARCHAR(255) NOT NULL,
    tenant_id VARCHAR(100) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ,
    email_verified BOOLEAN NOT NULL DEFAULT false,
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_users_tenant ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_active ON users(active);

-- User roles
CREATE TABLE IF NOT EXISTS user_roles (
    user_id VARCHAR(100) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL,
    PRIMARY KEY (user_id, role)
);

-- Memories
CREATE TABLE IF NOT EXISTS memories (
    id VARCHAR(100) PRIMARY KEY,
    content TEXT NOT NULL,
    summary VARCHAR(500),
    category VARCHAR(50),
    importance VARCHAR(50),
    validation_status VARCHAR(50) DEFAULT 'PENDING',
    embedding FLOAT4[],
    metadata JSONB,
    source_type VARCHAR(50),
    source_reference VARCHAR(500),
    created_by VARCHAR(100),
    tenant_id VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_accessed_at TIMESTAMPTZ,
    version INTEGER DEFAULT 1,
    access_count INTEGER DEFAULT 0,
    injection_count INTEGER DEFAULT 0,
    helpful_count INTEGER DEFAULT 0,
    not_helpful_count INTEGER DEFAULT 0,
    code_example TEXT,
    programming_language VARCHAR(50),
    memory_type VARCHAR(50) DEFAULT '',
    deleted_at TIMESTAMPTZ,
    emotional_weight DOUBLE PRECISION NOT NULL DEFAULT 0,
    sim_hash VARCHAR(32) DEFAULT '',
    valid_from TIMESTAMPTZ,
    valid_to TIMESTAMPTZ,
    decay_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
    superseded_by VARCHAR(100) DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_memories_tenant ON memories(tenant_id);
CREATE INDEX IF NOT EXISTS idx_memories_category ON memories(category);
CREATE INDEX IF NOT EXISTS idx_memories_importance ON memories(importance);
CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at);
CREATE INDEX IF NOT EXISTS idx_memories_sim_hash ON memories(sim_hash);
CREATE INDEX IF NOT EXISTS idx_memories_deleted_at ON memories(deleted_at);

-- Memory tags
CREATE TABLE IF NOT EXISTS memory_tags (
    memory_id VARCHAR(100) NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    tag VARCHAR(100) NOT NULL,
    PRIMARY KEY (memory_id, tag)
);

-- Memory relationships
CREATE TABLE IF NOT EXISTS memory_relationships (
    id VARCHAR(100) PRIMARY KEY,
    from_memory_id VARCHAR(100) NOT NULL,
    to_memory_id VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    frequency INTEGER DEFAULT 1,
    severity VARCHAR(20),
    strength DOUBLE PRECISION DEFAULT 0.5,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    tenant_id VARCHAR(100) NOT NULL
);

-- Memory versions
CREATE TABLE IF NOT EXISTS memory_versions (
    id VARCHAR(100) PRIMARY KEY,
    memory_id VARCHAR(100) NOT NULL,
    version INTEGER NOT NULL,
    content TEXT,
    summary VARCHAR(500),
    category VARCHAR(50),
    importance VARCHAR(50),
    metadata JSONB,
    code_example TEXT,
    changed_by VARCHAR(100),
    change_reason TEXT,
    change_type VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    tenant_id VARCHAR(100)
);

-- Memory version tags
CREATE TABLE IF NOT EXISTS memory_version_tags (
    memory_version_id VARCHAR(100) NOT NULL REFERENCES memory_versions(id) ON DELETE CASCADE,
    tag VARCHAR(100) NOT NULL,
    PRIMARY KEY (memory_version_id, tag)
);

-- Audit logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id VARCHAR(100) PRIMARY KEY,
    event_type VARCHAR(100),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id VARCHAR(100),
    session_id VARCHAR(100),
    user_request TEXT,
    decision JSONB,
    reasoning TEXT,
    confidence DOUBLE PRECISION,
    input_data JSONB,
    output_data JSONB,
    latency_ms INTEGER,
    llm_calls INTEGER,
    tokens_used INTEGER,
    outcome VARCHAR(50),
    error_message TEXT,
    user_feedback JSONB,
    tenant_id VARCHAR(100) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_tenant ON audit_logs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_event_type ON audit_logs(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_session ON audit_logs(session_id);

-- Audit log element collections
CREATE TABLE IF NOT EXISTS audit_memories_accessed (
    audit_log_id VARCHAR(100) NOT NULL REFERENCES audit_logs(id) ON DELETE CASCADE,
    memory_id VARCHAR(100) NOT NULL,
    PRIMARY KEY (audit_log_id, memory_id)
);

CREATE TABLE IF NOT EXISTS audit_memories_created (
    audit_log_id VARCHAR(100) NOT NULL REFERENCES audit_logs(id) ON DELETE CASCADE,
    memory_id VARCHAR(100) NOT NULL,
    PRIMARY KEY (audit_log_id, memory_id)
);

CREATE TABLE IF NOT EXISTS audit_memories_modified (
    audit_log_id VARCHAR(100) NOT NULL REFERENCES audit_logs(id) ON DELETE CASCADE,
    memory_id VARCHAR(100) NOT NULL,
    PRIMARY KEY (audit_log_id, memory_id)
);

-- Notes
CREATE TABLE IF NOT EXISTS notes (
    id VARCHAR(100) PRIMARY KEY,
    tenant_id VARCHAR(100) NOT NULL,
    session_id VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    title VARCHAR(500) NOT NULL,
    content TEXT NOT NULL,
    category VARCHAR(50) DEFAULT 'PROJECT_SPECIFIC',
    project_id VARCHAR(100),
    severity VARCHAR(50) DEFAULT 'MEDIUM',
    error_pattern TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_accessed_at TIMESTAMPTZ,
    last_occurrence_at TIMESTAMPTZ,
    access_count INTEGER DEFAULT 0,
    created_by VARCHAR(100),
    auto_generated BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_notes_tenant ON notes(tenant_id);
CREATE INDEX IF NOT EXISTS idx_notes_session ON notes(session_id);
CREATE INDEX IF NOT EXISTS idx_notes_project ON notes(project_id);
CREATE INDEX IF NOT EXISTS idx_notes_type ON notes(type);
CREATE INDEX IF NOT EXISTS idx_notes_category ON notes(category);
CREATE INDEX IF NOT EXISTS idx_notes_severity ON notes(severity);
CREATE INDEX IF NOT EXISTS idx_notes_created_at ON notes(created_at);

-- Note element collections
CREATE TABLE IF NOT EXISTS note_keywords (
    note_id VARCHAR(100) NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    keyword VARCHAR(100) NOT NULL,
    PRIMARY KEY (note_id, keyword)
);

CREATE TABLE IF NOT EXISTS note_related_memories (
    note_id VARCHAR(100) NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    memory_id VARCHAR(100) NOT NULL,
    PRIMARY KEY (note_id, memory_id)
);

CREATE TABLE IF NOT EXISTS note_related_notes (
    note_id VARCHAR(100) NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    related_note_id VARCHAR(100) NOT NULL,
    PRIMARY KEY (note_id, related_note_id)
);

-- Hindsight notes
CREATE TABLE IF NOT EXISTS hindsight_notes (
    id VARCHAR(100) PRIMARY KEY,
    tenant_id VARCHAR(100) NOT NULL,
    session_id VARCHAR(100),
    title VARCHAR(500),
    error_pattern TEXT,
    severity VARCHAR(50) DEFAULT 'MEDIUM',
    last_accessed_at TIMESTAMPTZ,
    access_count INTEGER DEFAULT 0,
    error_type VARCHAR(100),
    error_message TEXT,
    error_context TEXT,
    resolution TEXT,
    resolution_steps TEXT,
    resolution_reference VARCHAR(500),
    lessons_learned TEXT,
    prevention_strategy TEXT,
    occurrence_count INTEGER DEFAULT 1,
    reference_count INTEGER DEFAULT 0,
    prevention_success_count INTEGER DEFAULT 0,
    created_by VARCHAR(100),
    auto_generated BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_occurrence_at TIMESTAMPTZ,
    prevention_verified BOOLEAN NOT NULL DEFAULT false,
    priority VARCHAR(20)
);

CREATE INDEX IF NOT EXISTS idx_hindsight_tenant ON hindsight_notes(tenant_id);
CREATE INDEX IF NOT EXISTS idx_hindsight_session ON hindsight_notes(session_id);
CREATE INDEX IF NOT EXISTS idx_hindsight_error_type ON hindsight_notes(error_type);
CREATE INDEX IF NOT EXISTS idx_hindsight_created_at ON hindsight_notes(created_at);
CREATE INDEX IF NOT EXISTS idx_hindsight_severity ON hindsight_notes(severity);

-- Hindsight element collections
CREATE TABLE IF NOT EXISTS hindsight_tags (
    hindsight_note_id VARCHAR(100) NOT NULL REFERENCES hindsight_notes(id) ON DELETE CASCADE,
    tag VARCHAR(100) NOT NULL,
    PRIMARY KEY (hindsight_note_id, tag)
);

CREATE TABLE IF NOT EXISTS hindsight_related_memories (
    hindsight_note_id VARCHAR(100) NOT NULL REFERENCES hindsight_notes(id) ON DELETE CASCADE,
    memory_id VARCHAR(100) NOT NULL,
    PRIMARY KEY (hindsight_note_id, memory_id)
);

CREATE TABLE IF NOT EXISTS hindsight_related_notes (
    hindsight_note_id VARCHAR(100) NOT NULL REFERENCES hindsight_notes(id) ON DELETE CASCADE,
    note_id VARCHAR(100) NOT NULL,
    PRIMARY KEY (hindsight_note_id, note_id)
);

-- Context summaries
-- DERIVED: rebuild --compress
CREATE TABLE IF NOT EXISTS context_summaries (
    id VARCHAR(100) PRIMARY KEY,
    tenant_id VARCHAR(100) NOT NULL,
    session_id VARCHAR(100) NOT NULL,
    original_token_count INTEGER NOT NULL,
    compressed_token_count INTEGER NOT NULL,
    compression_ratio DOUBLE PRECISION NOT NULL,
    summary TEXT,
    recent_window_size INTEGER NOT NULL DEFAULT 10,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    model_used VARCHAR(100),
    compression_method VARCHAR(50)
);

CREATE INDEX IF NOT EXISTS idx_context_summary_tenant ON context_summaries(tenant_id);
CREATE INDEX IF NOT EXISTS idx_context_summary_session ON context_summaries(session_id);
CREATE INDEX IF NOT EXISTS idx_context_summary_created_at ON context_summaries(created_at);

-- Context summary element collections
-- DERIVED: rebuild --compress
CREATE TABLE IF NOT EXISTS context_summary_goals (
    context_summary_id VARCHAR(100) NOT NULL REFERENCES context_summaries(id) ON DELETE CASCADE,
    goal TEXT NOT NULL
);

-- DERIVED: rebuild --compress
CREATE TABLE IF NOT EXISTS context_summary_decisions (
    context_summary_id VARCHAR(100) NOT NULL REFERENCES context_summaries(id) ON DELETE CASCADE,
    decision TEXT NOT NULL
);

-- DERIVED: rebuild --compress
CREATE TABLE IF NOT EXISTS context_summary_errors (
    context_summary_id VARCHAR(100) NOT NULL REFERENCES context_summaries(id) ON DELETE CASCADE,
    error TEXT NOT NULL
);

-- DERIVED: rebuild --compress
CREATE TABLE IF NOT EXISTS context_summary_todos (
    context_summary_id VARCHAR(100) NOT NULL REFERENCES context_summaries(id) ON DELETE CASCADE,
    todo TEXT NOT NULL
);

-- Insert default tenant
INSERT INTO tenants (id, name, slug, description, active)
VALUES ('a9f814d2-4dae-41f3-851b-8aa3d4706561', 'Default', 'default', 'Default tenant', true)
ON CONFLICT (id) DO NOTHING;
