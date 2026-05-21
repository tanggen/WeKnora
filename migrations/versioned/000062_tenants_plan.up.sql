-- 000062: Tenants - Add plan_id and trial_expires_at

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS plan_id VARCHAR(32) DEFAULT 'trial';
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS trial_expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_tenants_plan_id ON tenants(plan_id);
