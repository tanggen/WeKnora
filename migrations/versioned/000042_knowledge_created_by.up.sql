-- Migration: Add created_by column to knowledge table
-- Tracks which user created each knowledge entry (for ownership/permission checks)

ALTER TABLE knowledge
    ADD COLUMN IF NOT EXISTS created_by VARCHAR(36);

-- Index for filtering knowledge by creator
CREATE INDEX IF NOT EXISTS idx_knowledge_created_by ON knowledge (created_by);
