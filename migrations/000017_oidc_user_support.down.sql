-- Revert migration 000017: remove OIDC columns and restore NOT NULL on password_hash.
DROP INDEX IF EXISTS users_oidc_subject_provider_idx;

ALTER TABLE users DROP COLUMN IF EXISTS display_name;
ALTER TABLE users DROP COLUMN IF EXISTS oidc_provider;
ALTER TABLE users DROP COLUMN IF EXISTS oidc_subject;

-- NOTE: this will fail if any OIDC-only accounts (password_hash IS NULL) exist.
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;