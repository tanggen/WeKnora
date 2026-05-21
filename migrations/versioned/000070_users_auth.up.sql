-- Migration 000070: Auth enhancements for SaaS multi-tenant
-- Adds unique partial index for phone column
-- Note: phone and password_expired columns already exist in the application model

-- Create partial unique index on phone (only for non-null, non-empty values)
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone
    ON users(phone) WHERE phone IS NOT NULL AND phone != '';

-- Update seed admin user: set password_expired = TRUE for default admin password
UPDATE users SET password_expired = TRUE
    WHERE username = 'admin' AND password_expired = FALSE;
