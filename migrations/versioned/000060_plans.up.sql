-- 000060: Plans and Plan Configs
-- Create the subscription plan management tables

CREATE TABLE IF NOT EXISTS plans (
    id             VARCHAR(32) PRIMARY KEY,        -- e.g. 'trial', 'basic', 'pro', 'enterprise'
    name           VARCHAR(128) NOT NULL,
    description    TEXT,
    is_trial       BOOLEAN NOT NULL DEFAULT FALSE,
    trial_days     INT NOT NULL DEFAULT 0,
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order     INT NOT NULL DEFAULT 99,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_plans_is_active ON plans(is_active);
CREATE INDEX IF NOT EXISTS idx_plans_sort_order ON plans(sort_order);

CREATE TABLE IF NOT EXISTS plan_configs (
    plan_id                 VARCHAR(32) PRIMARY KEY REFERENCES plans(id) ON DELETE CASCADE,

    -- Resource limits (0 = unlimited)
    max_users               INT NOT NULL DEFAULT 5,
    max_knowledge_bases     INT NOT NULL DEFAULT 20,
    max_knowledge_per_kb    INT NOT NULL DEFAULT 1000,
    max_chunks_per_knowledge INT NOT NULL DEFAULT 10000,
    storage_quota_bytes     BIGINT NOT NULL DEFAULT 10737418240,  -- 10 GB
    token_quota_monthly     BIGINT NOT NULL DEFAULT 100000,       -- 100K tokens/month
    max_agents              INT NOT NULL DEFAULT 5,

    -- Pricing (CNY cents)
    price_monthly_cny       INT NOT NULL DEFAULT 0,
    price_yearly_cny        INT NOT NULL DEFAULT 0,

    -- JSON configuration
    features                JSONB NOT NULL DEFAULT '["chat","search"]',
    marketing_features      JSONB NOT NULL DEFAULT '[]',

    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
