-- +goose Up
-- Audio features of uploaded masters (specs/workers/audio-analysis.md), one
-- row per upload: a re-upload of a track gets its own row, the latest
-- uploaded_at is the current master. track_id references Catalog by ID only.
-- bpm is NULL when the track has no detectable tempo; levels are floored at
-- -70 (the EBU R128 absolute gate).
CREATE TABLE audio_features.track_features (
    upload_id         uuid             PRIMARY KEY,
    track_id          uuid             NOT NULL,
    bpm               double precision CHECK (bpm BETWEEN 60 AND 200),
    bpm_confidence    double precision NOT NULL CHECK (bpm_confidence BETWEEN 0 AND 1),
    integrated_lufs   double precision NOT NULL CHECK (integrated_lufs >= -70),
    loudness_range_lu double precision NOT NULL CHECK (loudness_range_lu >= 0),
    true_peak_dbtp    double precision NOT NULL CHECK (true_peak_dbtp >= -70),
    silence_ratio     double precision NOT NULL CHECK (silence_ratio BETWEEN 0 AND 1),
    analyzed_ms       bigint           NOT NULL CHECK (analyzed_ms > 0),
    analyzer_version  text             NOT NULL,
    source_sha256     text             NOT NULL,
    uploaded_at       timestamptz      NOT NULL,
    analyzed_at       timestamptz      NOT NULL
);

CREATE INDEX track_features_track_idx ON audio_features.track_features (track_id, uploaded_at DESC);

-- Transactional outbox (libs/platform/outbox): audio.features_extracted is
-- written with its row and relayed to media.events.
CREATE TABLE audio_features.outbox (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    topic      text        NOT NULL,
    key        bytea,
    value      bytea       NOT NULL,
    headers    jsonb       NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE audio_features.outbox;
DROP TABLE audio_features.track_features;
