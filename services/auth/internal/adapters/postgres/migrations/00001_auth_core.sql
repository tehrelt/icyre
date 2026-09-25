-- +goose Up
-- Credentials and sessions. Passwords only as Argon2id PHC strings,
-- refresh tokens only as SHA-256 hashes.

CREATE TABLE auth.accounts (
    id            uuid        PRIMARY KEY,
    email         text        NOT NULL CHECK (email = lower(email) AND char_length(email) BETWEEN 3 AND 254),
    password_hash text        NOT NULL CHECK (password_hash LIKE '$argon2id$%'),
    roles         text[]      NOT NULL DEFAULT '{USER}',
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL,
    CONSTRAINT accounts_email_key UNIQUE (email)
);

CREATE TABLE auth.sessions (
    id            uuid        PRIMARY KEY,
    user_id       uuid        NOT NULL REFERENCES auth.accounts (id) ON DELETE CASCADE,
    refresh_hash  bytea       NOT NULL CHECK (octet_length(refresh_hash) = 32),
    previous_hash bytea       CHECK (octet_length(previous_hash) = 32),
    user_agent    text        NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL,
    last_used_at  timestamptz NOT NULL,
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz,
    revoke_reason text        CHECK (revoke_reason IN ('logout', 'revoked', 'token_reuse')),
    CONSTRAINT sessions_refresh_hash_key UNIQUE (refresh_hash)
);

CREATE INDEX sessions_previous_hash_idx ON auth.sessions (previous_hash) WHERE previous_hash IS NOT NULL;
-- Active sessions of a user (GET /auth/sessions).
CREATE INDEX sessions_user_active_idx ON auth.sessions (user_id, created_at DESC) WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE auth.sessions;
DROP TABLE auth.accounts;
