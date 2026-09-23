-- ============================================================================
-- TeslaCost User Language Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- The UI language is chosen client-side (localStorage), but background jobs (reminder webhooks,
-- sync failure alerts) build their messages outside any HTTP request and need a stored preference.
ALTER TABLE users ADD COLUMN language TEXT NOT NULL DEFAULT 'en' CHECK (language IN ('en', 'fr'));
