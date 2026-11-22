-- +goose Up
-- Account blocking, moderation idempotency receipts, and account_blocked session revocation reason.

ALTER TABLE auth.accounts
    ADD COLUMN blocked boolean NOT NULL DEFAULT false,
    ADD COLUMN blocked_at timestamptz;

ALTER TABLE auth.sessions
    DROP CONSTRAINT IF EXISTS sessions_revoke_reason_check;

ALTER TABLE auth.sessions
    ADD CONSTRAINT sessions_revoke_reason_check
        CHECK (revoke_reason IN ('logout', 'revoked', 'token_reuse', 'account_blocked'));

CREATE TABLE auth.moderation_receipts (
    operation_id     uuid        PRIMARY KEY,
    user_id          uuid        NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
    blocked          boolean     NOT NULL,
    changed          boolean     NOT NULL,
    revoked_sessions integer     NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auth.pending_revocations (
    session_id uuid        PRIMARY KEY,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS auth.pending_revocations;
DROP TABLE IF EXISTS auth.moderation_receipts;

ALTER TABLE auth.sessions
    DROP CONSTRAINT IF EXISTS sessions_revoke_reason_check;

ALTER TABLE auth.sessions
    ADD CONSTRAINT sessions_revoke_reason_check
        CHECK (revoke_reason IN ('logout', 'revoked', 'token_reuse'));

ALTER TABLE auth.accounts
    DROP COLUMN IF EXISTS blocked_at,
    DROP COLUMN IF EXISTS blocked;
