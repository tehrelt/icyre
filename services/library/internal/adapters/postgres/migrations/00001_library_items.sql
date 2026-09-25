-- +goose Up
-- One row per saved track/album (specs/services/library.md data model).
-- entity_id references Catalog data by ID only: no cross-schema foreign keys.
CREATE TABLE library.items (
    user_id   uuid        NOT NULL,
    kind      text        NOT NULL CHECK (kind IN ('track', 'album')),
    entity_id uuid        NOT NULL,
    saved_at  timestamptz NOT NULL,
    PRIMARY KEY (user_id, kind, entity_id)
);

-- Newest-first keyset pagination per user and kind.
CREATE INDEX items_recent_idx ON library.items (user_id, kind, saved_at DESC, entity_id DESC);

-- +goose Down
DROP TABLE library.items;
