-- 000062: Tenants - Remove plan_id and trial_expires_at (rollback)

DROP INDEX IF EXISTS idx_tenants_plan_id;
ALTER TABLE tenants DROP COLUMN IF EXISTS trial_expires_at;
ALTER TABLE tenants DROP COLUMN IF EXISTS plan_id;
