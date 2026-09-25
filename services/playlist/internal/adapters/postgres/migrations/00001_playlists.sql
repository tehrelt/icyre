-- +goose Up
CREATE TABLE playlist.playlists (
    id         uuid        PRIMARY KEY,
    owner_id   uuid        NOT NULL,
    title      text        NOT NULL CHECK (char_length(title) BETWEEN 1 AND 100),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX playlists_owner_idx ON playlist.playlists (owner_id, updated_at DESC);

-- PlaylistTrack (specs/services/playlist.md). A track appears once per
-- playlist in this slice; track_id references Catalog by ID only.
CREATE TABLE playlist.playlist_tracks (
    playlist_id uuid        NOT NULL REFERENCES playlist.playlists (id) ON DELETE CASCADE,
    track_id    uuid        NOT NULL,
    position    int         NOT NULL CHECK (position > 0),
    added_by    uuid        NOT NULL,
    added_at    timestamptz NOT NULL,
    PRIMARY KEY (playlist_id, track_id),
    CONSTRAINT playlist_tracks_position_key UNIQUE (playlist_id, position)
);

-- +goose Down
DROP TABLE playlist.playlist_tracks;
DROP TABLE playlist.playlists;
