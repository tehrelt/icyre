-- +goose Up
CREATE TABLE profile.avatar_uploads (
    id             uuid        PRIMARY KEY,
    user_id        uuid        NOT NULL REFERENCES profile.profiles(user_id) ON DELETE CASCADE,
    content_type   text        NOT NULL,
    size_bytes     bigint      NOT NULL CHECK (size_bytes > 0),
    sha256         bytea       NOT NULL CHECK (length(sha256) = 32),
    object_key     text        NOT NULL UNIQUE,
    status         text        NOT NULL CHECK (status IN ('PENDING', 'COMPLETED', 'FAILED')),
    failure_reason text        NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL,
    expires_at     timestamptz NOT NULL,
    completed_at   timestamptz,
    CHECK ((status = 'FAILED') = (failure_reason <> '')),
    CHECK ((status = 'PENDING') = (completed_at IS NULL))
);

CREATE INDEX avatar_uploads_user_idx ON profile.avatar_uploads (user_id, created_at DESC);

-- +goose Down
DROP TABLE profile.avatar_uploads;
