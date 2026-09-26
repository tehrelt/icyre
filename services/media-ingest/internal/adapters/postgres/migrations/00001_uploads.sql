-- +goose Up
-- Upload sessions (specs/services/media-ingest.md). track_id and uploader_id
-- reference Catalog and Auth data by ID only: no cross-schema foreign keys.
CREATE TABLE media.uploads (
    id             uuid        PRIMARY KEY,
    track_id       uuid        NOT NULL,
    uploader_id    uuid        NOT NULL,
    content_type   text        NOT NULL,
    size_bytes     bigint      NOT NULL CHECK (size_bytes > 0),
    sha256         bytea       NOT NULL CHECK (length(sha256) = 32),
    object_key     text        NOT NULL UNIQUE,
    status         text        NOT NULL CHECK (status IN ('PENDING', 'COMPLETED', 'FAILED')),
    failure_reason text,
    created_at     timestamptz NOT NULL,
    expires_at     timestamptz NOT NULL,
    completed_at   timestamptz,
    CHECK ((status = 'FAILED') = (failure_reason IS NOT NULL)),
    CHECK ((status = 'PENDING') = (completed_at IS NULL))
);

CREATE INDEX uploads_track_idx ON media.uploads (track_id, created_at DESC);

-- Transactional outbox (libs/platform/outbox): media.events messages are
-- written here in the transaction of the status change and relayed to Kafka.
CREATE TABLE media.outbox (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    topic      text        NOT NULL,
    key        bytea,
    value      bytea       NOT NULL,
    headers    jsonb       NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE media.outbox;
DROP TABLE media.uploads;
