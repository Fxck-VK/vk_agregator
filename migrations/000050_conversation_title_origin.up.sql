-- Preserve whether a conversation title came from the account holder or the
-- internal title worker. The field is deliberately not exposed by web DTOs.

ALTER TABLE conversations
    ADD COLUMN title_origin TEXT NOT NULL DEFAULT 'manual';

ALTER TABLE conversations
    ADD CONSTRAINT conversations_title_origin_check
        CHECK (title_origin IN ('manual', 'auto_pending', 'auto_fallback', 'auto_generated'));

-- Existing blank Web conversations are the only historical rows eligible for
-- the new automatic title flow. Every named and non-Web row remains manual.
UPDATE conversations
SET title_origin = 'auto_pending'
WHERE source = 'web'
  AND btrim(title) = '';
