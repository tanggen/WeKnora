-- Migration: Create tenant_users table for multi-tenant user-role associations
-- This table maps users to tenants with a specific role and custom permissions.

CREATE TABLE IF NOT EXISTS tenant_users (
    tenant_id   BIGINT       NOT NULL,
    user_id     VARCHAR(36)  NOT NULL,
    role        VARCHAR(32)  NOT NULL DEFAULT 'viewer',
    permissions JSONB        NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, user_id)
);

-- Index for looking up all users in a tenant
CREATE INDEX IF NOT EXISTS idx_tenant_users_tenant_id ON tenant_users (tenant_id);

-- Index for looking up a user's tenant association(s)
CREATE INDEX IF NOT EXISTS idx_tenant_users_user_id ON tenant_users (user_id);

-- Index for role-based queries within a tenant
CREATE INDEX IF NOT EXISTS idx_tenant_users_role ON tenant_users (tenant_id, role);

-- Index for soft-delete queries
CREATE INDEX IF NOT EXISTS idx_tenant_users_deleted_at ON tenant_users (deleted_at);
