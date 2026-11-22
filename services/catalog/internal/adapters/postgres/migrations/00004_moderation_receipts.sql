-- +goose Up
CREATE TABLE catalog.moderation_receipts (
    operation_id    uuid        PRIMARY KEY,
    track_id        uuid        NOT NULL REFERENCES catalog.tracks(id) ON DELETE CASCADE,
    previous_status text        NOT NULL,
    current_status  text        NOT NULL,
    changed         boolean     NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE catalog.moderation_receipts;
