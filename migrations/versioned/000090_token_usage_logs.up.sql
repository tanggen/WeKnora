-- 000090: Token usage logs table (Phase 2, optional)
-- Stores per-request token consumption for detailed audit and model-level reporting.
CREATE TABLE IF NOT EXISTS token_usage_logs (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id     VARCHAR(36) NOT NULL,
    session_id  VARCHAR(36),
    model_name  VARCHAR(64) NOT NULL,
    tokens_in   INT NOT NULL DEFAULT 0,
    tokens_out  INT NOT NULL DEFAULT 0,
    tokens_total INT GENERATED ALWAYS AS (tokens_in + tokens_out) STORED,
    endpoint    VARCHAR(128),
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_token_logs_tenant_date ON token_usage_logs(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_token_logs_user ON token_usage_logs(tenant_id, user_id);
