-- +goose Up
-- One row per completed listen. playback_id is the idempotency key: a
-- redelivered playback event inserts nothing.
CREATE TABLE history.listens (
    playback_id uuid        PRIMARY KEY,
    user_id     uuid        NOT NULL,
    track_id    uuid        NOT NULL,
    source      text        NOT NULL DEFAULT '',
    duration_ms bigint      NOT NULL CHECK (duration_ms > 0),
    listened_ms bigint      NOT NULL CHECK (listened_ms >= 0),
    played_at   timestamptz NOT NULL
);

CREATE INDEX listens_recent_idx ON history.listens (user_id, played_at DESC, playback_id DESC);

-- +goose Down
DROP TABLE history.listens;
