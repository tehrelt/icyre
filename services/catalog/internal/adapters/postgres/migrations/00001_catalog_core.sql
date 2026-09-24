-- +goose Up
-- Catalog domain lives in its own schema; other services never read it directly.

CREATE TABLE catalog.artists (
    id         uuid        PRIMARY KEY,
    name       text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE catalog.genres (
    id   uuid PRIMARY KEY,
    slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100)
);

CREATE TABLE catalog.albums (
    id           uuid        PRIMARY KEY,
    title        text        NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    album_type   text        NOT NULL CHECK (album_type IN ('ALBUM', 'EP', 'SINGLE', 'COMPILATION')),
    release_date date        NOT NULL,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL
);

-- Keyset pagination: ORDER BY release_date DESC, id DESC.
CREATE INDEX albums_release_idx ON catalog.albums (release_date DESC, id DESC);

CREATE TABLE catalog.album_artists (
    album_id  uuid     NOT NULL REFERENCES catalog.albums (id) ON DELETE CASCADE,
    artist_id uuid     NOT NULL REFERENCES catalog.artists (id) ON DELETE RESTRICT,
    position  smallint NOT NULL CHECK (position >= 0),
    PRIMARY KEY (album_id, artist_id)
);

CREATE INDEX album_artists_artist_idx ON catalog.album_artists (artist_id);

CREATE TABLE catalog.album_genres (
    album_id uuid NOT NULL REFERENCES catalog.albums (id) ON DELETE CASCADE,
    genre_id uuid     NOT NULL REFERENCES catalog.genres (id) ON DELETE RESTRICT,
    position smallint NOT NULL CHECK (position >= 0),
    PRIMARY KEY (album_id, genre_id)
);

CREATE INDEX album_genres_genre_idx ON catalog.album_genres (genre_id);

CREATE TABLE catalog.tracks (
    id           uuid        PRIMARY KEY,
    album_id     uuid        NOT NULL REFERENCES catalog.albums (id) ON DELETE RESTRICT,
    title        text        NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    duration_ms  integer     NOT NULL CHECK (duration_ms > 0 AND duration_ms <= 86400000),
    track_number smallint    NOT NULL CHECK (track_number BETWEEN 1 AND 999),
    disc_number  smallint    NOT NULL DEFAULT 1 CHECK (disc_number BETWEEN 1 AND 99),
    explicit     boolean     NOT NULL DEFAULT false,
    isrc         text        CHECK (isrc ~ '^[A-Z]{2}[A-Z0-9]{3}[0-9]{7}$'),
    status       text        NOT NULL CHECK (status IN ('DRAFT', 'PROCESSING', 'READY', 'BLOCKED', 'DELETED')),
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL
);

-- One live track per album position; deleted tracks free their slot.
CREATE UNIQUE INDEX tracks_album_position_key
    ON catalog.tracks (album_id, disc_number, track_number)
    WHERE status <> 'DELETED';

-- Partial indexes skip deleted rows (specs/data/postgres.md).
CREATE INDEX tracks_isrc_idx ON catalog.tracks (isrc) WHERE isrc IS NOT NULL AND status <> 'DELETED';
CREATE INDEX tracks_status_idx ON catalog.tracks (status) WHERE status <> 'DELETED';

CREATE TABLE catalog.track_artists (
    track_id  uuid     NOT NULL REFERENCES catalog.tracks (id) ON DELETE CASCADE,
    artist_id uuid     NOT NULL REFERENCES catalog.artists (id) ON DELETE RESTRICT,
    position  smallint NOT NULL CHECK (position >= 0),
    PRIMARY KEY (track_id, artist_id)
);

CREATE INDEX track_artists_artist_idx ON catalog.track_artists (artist_id);

-- +goose Down
DROP TABLE catalog.track_artists;
DROP TABLE catalog.tracks;
DROP TABLE catalog.album_genres;
DROP TABLE catalog.album_artists;
DROP TABLE catalog.albums;
DROP TABLE catalog.genres;
DROP TABLE catalog.artists;
