-- Revert migration 000016: restore NOT NULL on data, drop storage_path.
ALTER TABLE expense_documents
    ALTER COLUMN data SET NOT NULL;

ALTER TABLE expense_documents
    DROP COLUMN IF EXISTS storage_path;