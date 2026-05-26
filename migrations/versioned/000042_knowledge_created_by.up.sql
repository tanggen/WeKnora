-- Migration: Add created_by column to knowledges table
-- Tracks which user created each knowledge entry (for ownership/permission checks)

ALTER TABLE knowledges
    ADD COLUMN IF NOT EXISTS created_by VARCHAR(36);

-- Index for filtering knowledge by creator
CREATE INDEX IF NOT EXISTS idx_knowledges_created_by ON knowledges (created_by);
