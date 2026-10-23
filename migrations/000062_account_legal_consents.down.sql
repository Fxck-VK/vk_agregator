-- Development rollback only: destroys legal evidence. Never run automatically.
DROP TABLE IF EXISTS account_legal_events;
DROP TABLE IF EXISTS account_legal_revisions;
