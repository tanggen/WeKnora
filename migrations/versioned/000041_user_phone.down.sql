-- Rollback: Remove phone and password_expired columns from users table

ALTER TABLE users
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS password_expired;
