-- Migration: Add phone and password_expired columns to users table
-- phone: optional phone number for user identification (globally unique when set)
-- password_expired: flag indicating user must change password on next login

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS phone VARCHAR(20),
    ADD COLUMN IF NOT EXISTS password_expired BOOLEAN NOT NULL DEFAULT FALSE;

-- Index for phone lookup
CREATE INDEX IF NOT EXISTS idx_users_phone ON users (phone) WHERE phone IS NOT NULL AND phone != '';
