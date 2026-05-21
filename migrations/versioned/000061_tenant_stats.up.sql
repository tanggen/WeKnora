-- 000061: Tenant Statistics
-- Centralized statistics table for tenant usage tracking

CREATE TABLE IF NOT EXISTS tenant_stats (
    tenant_id             BIGINT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,

    -- Counters
    user_count            INT NOT NULL DEFAULT 0,
    knowledge_base_count  INT NOT NULL DEFAULT 0,
    knowledge_count       INT NOT NULL DEFAULT 0,
    chunk_count           BIGINT NOT NULL DEFAULT 0,
    storage_used_bytes    BIGINT NOT NULL DEFAULT 0,
    token_used_total      BIGINT NOT NULL DEFAULT 0,        -- all-time total (never resets)
    token_used_monthly    BIGINT NOT NULL DEFAULT 0,        -- current month only
    token_quota_reset_at  TIMESTAMPTZ,                      -- next reset timestamp

    agent_count           INT NOT NULL DEFAULT 0,
    session_count         BIGINT NOT NULL DEFAULT 0,

    -- Plan snapshot
    plan_id               VARCHAR(32),
    storage_quota_bytes    BIGINT NOT NULL DEFAULT 10737418240,

    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenant_stats_plan_id ON tenant_stats(plan_id);
