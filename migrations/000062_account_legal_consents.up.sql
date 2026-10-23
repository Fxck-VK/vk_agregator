-- Additive evidence only. This migration does not accept terms or enroll users.
CREATE TABLE account_legal_revisions (
 checksum TEXT PRIMARY KEY CHECK(checksum ~ '^[a-f0-9]{64}$'),
 version TEXT NOT NULL,
 snapshot JSONB NOT NULL CHECK(jsonb_typeof(snapshot)='object'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE account_legal_events (
 account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
 request_id UUID NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('accept','marketing_grant','marketing_revoke')),
 document_version TEXT NOT NULL,
 document_checksum TEXT NOT NULL REFERENCES account_legal_revisions(checksum),
 marketing_version TEXT NOT NULL,
 marketing_checksum TEXT NOT NULL CHECK(marketing_checksum ~ '^[a-f0-9]{64}$'),
 marketing_opt_in BOOLEAN NOT NULL,
 recipient_hash TEXT NOT NULL DEFAULT '' CHECK(recipient_hash='' OR recipient_hash ~ '^[a-f0-9]{64}$'),
 locale TEXT NOT NULL CHECK(locale IN ('ru','en')),
 request_fingerprint TEXT NOT NULL CHECK(request_fingerprint ~ '^[a-f0-9]{64}$'),
 sequence BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,request_id),
 CHECK((NOT marketing_opt_in AND recipient_hash='') OR (marketing_opt_in AND recipient_hash<>'')),
 CHECK((kind='marketing_grant' AND marketing_opt_in) OR (kind='marketing_revoke' AND NOT marketing_opt_in) OR kind='accept')
);
CREATE INDEX account_legal_events_acceptance_idx ON account_legal_events(account_id,document_checksum) WHERE kind='accept';
CREATE INDEX account_legal_events_marketing_idx ON account_legal_events(account_id,sequence DESC) WHERE kind<>'accept' OR marketing_opt_in;
COMMENT ON TABLE account_legal_events IS 'Account-owned legal evidence, no raw contacts/IP/content. Marketing sender must recheck current recipient and consent version.';
