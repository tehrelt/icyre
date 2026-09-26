-- +goose Up
-- Technical metadata of uploaded masters (specs/workers/metadata.md), one
-- row per upload: a re-upload of a track gets its own row, the latest
-- uploaded_at is the current master. track_id references Catalog by ID only.
CREATE TABLE media_metadata.track_metadata (
    upload_id      uuid        PRIMARY KEY,
    track_id       uuid        NOT NULL,
    container      text        NOT NULL,
    codec          text        NOT NULL,
    duration_ms    bigint      NOT NULL CHECK (duration_ms > 0),
    bitrate_bps    bigint      NOT NULL CHECK (bitrate_bps >= 0),
    sample_rate_hz integer     NOT NULL CHECK (sample_rate_hz > 0),
    channels       smallint    NOT NULL CHECK (channels > 0),
    source_sha256  text        NOT NULL,
    uploaded_at    timestamptz NOT NULL,
    extracted_at   timestamptz NOT NULL
);

CREATE INDEX track_metadata_track_idx ON media_metadata.track_metadata (track_id, uploaded_at DESC);

-- Transactional outbox (libs/platform/outbox): media.metadata_extracted is
-- written with its row and relayed to media.events.
CREATE TABLE media_metadata.outbox (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    topic      text        NOT NULL,
    key        bytea,
    value      bytea       NOT NULL,
    headers    jsonb       NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE media_metadata.outbox;
DROP TABLE media_metadata.track_metadata;
