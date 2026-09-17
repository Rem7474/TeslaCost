-- Migration 000017: Add OIDC/SSO support to users table.
-- password_hash becomes nullable (OIDC accounts have no local password).
-- oidc_subject + oidc_provider enable JIT provisioning and identity tracking.

ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

ALTER TABLE users ADD COLUMN IF NOT EXISTS oidc_subject  VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS oidc_provider VARCHAR(100);
ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name  VARCHAR(255);

-- One (provider, subject) pair maps to exactly one user account.
CREATE UNIQUE INDEX IF NOT EXISTS users_oidc_subject_provider_idx
    ON users(oidc_provider, oidc_subject)
    WHERE oidc_subject IS NOT NULL;