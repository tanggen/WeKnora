-- Rollback: Remove created_by column from knowledges table

ALTER TABLE knowledges
    DROP COLUMN IF EXISTS created_by;

DROP INDEX IF EXISTS idx_knowledges_created_by;
