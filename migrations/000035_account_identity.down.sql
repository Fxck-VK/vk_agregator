-- 000035_account_identity.down.sql

BEGIN;

DROP TABLE IF EXISTS account_links_audit;
DROP TABLE IF EXISTS account_credentials;
DROP TABLE IF EXISTS account_sessions;
DROP TABLE IF EXISTS account_identities;
DROP TABLE IF EXISTS accounts;

COMMIT;
