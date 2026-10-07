-- Private snapshots expire after seven days; delivered rows after one day.
CREATE TABLE account_security_notices (
 id uuid PRIMARY KEY,
 account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK (kind IN ('password_changed','password_reset','backup_email_added','backup_email_replaced','email_removed')),
 recipient text NOT NULL,
 event_at timestamptz NOT NULL,
 next_attempt_at timestamptz NOT NULL,
 attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
 lease_token uuid,
 lease_until timestamptz,
 sent_at timestamptz,
 CHECK ((lease_token IS NULL) = (lease_until IS NULL))
);
CREATE INDEX account_security_notices_pending ON account_security_notices(next_attempt_at, id) WHERE sent_at IS NULL;
CREATE INDEX account_security_notices_retention ON account_security_notices(event_at);
