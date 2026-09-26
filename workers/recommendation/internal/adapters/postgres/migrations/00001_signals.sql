-- +goose Up
-- Taste signals the worker keeps from other services' events
-- (specs/workers/recommendation-worker.md). IDs reference other services by
-- value only: no cross-schema foreign keys.

-- library.track_saved / library.track_removed, last writer wins by event
-- time: a removal keeps the row (liked = false) as a tombstone, so a stale
-- save delivered after it (redelivery, DLQ redrive) cannot resurrect the like.
CREATE TABLE recommendation.liked_tracks (
    user_id    uuid        NOT NULL,
    track_id   uuid        NOT NULL,
    liked      boolean     NOT NULL,
    changed_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, track_id)
);

-- library.album_saved / library.album_removed, same rules.
CREATE TABLE recommendation.liked_albums (
    user_id    uuid        NOT NULL,
    album_id   uuid        NOT NULL,
    liked      boolean     NOT NULL,
    changed_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, album_id)
);

-- audio.features_extracted: the features of each track's latest master.
-- bpm is NULL when the track has no detectable tempo.
CREATE TABLE recommendation.track_features (
    track_id          uuid             PRIMARY KEY,
    bpm               double precision,
    integrated_lufs   double precision NOT NULL,
    loudness_range_lu double precision NOT NULL,
    silence_ratio     double precision NOT NULL,
    analyzer_version  text             NOT NULL,
    uploaded_at       timestamptz      NOT NULL
);

-- +goose Down
DROP TABLE recommendation.track_features;
DROP TABLE recommendation.liked_albums;
DROP TABLE recommendation.liked_tracks;
