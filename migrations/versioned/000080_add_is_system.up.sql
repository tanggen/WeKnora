-- 000080: Add is_system column to tenants
-- Supports system tenant identification (ID=1, immutable)
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT FALSE;
