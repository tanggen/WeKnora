-- Rollback: Remove created_by column from knowledge table

ALTER TABLE knowledge
    DROP COLUMN IF EXISTS created_by;
