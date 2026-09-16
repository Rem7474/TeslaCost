-- Migration 000016: Replace BYTEA storage with filesystem storage path.
-- The binary data column is made nullable (backward-compatible) and a new
-- storage_path column is added. Existing rows keep their binary data until
-- an operator runs the optional data-migration script.

ALTER TABLE expense_documents
    ADD COLUMN IF NOT EXISTS storage_path VARCHAR(500);

-- Make data column nullable so new rows inserted without binary data are valid.
ALTER TABLE expense_documents
    ALTER COLUMN data DROP NOT NULL;