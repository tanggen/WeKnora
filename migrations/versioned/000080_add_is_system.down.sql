-- 000080: Rollback - Drop is_system column from tenants
ALTER TABLE tenants DROP COLUMN IF EXISTS is_system;
