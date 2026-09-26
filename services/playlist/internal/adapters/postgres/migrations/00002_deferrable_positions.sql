-- +goose Up
-- Reorder rewrites every position in one statement; positions are only
-- unique once the transaction commits, so the check is deferred.
ALTER TABLE playlist.playlist_tracks
    DROP CONSTRAINT playlist_tracks_position_key,
    ADD CONSTRAINT playlist_tracks_position_key UNIQUE (playlist_id, position) DEFERRABLE INITIALLY DEFERRED;

-- +goose Down
ALTER TABLE playlist.playlist_tracks
    DROP CONSTRAINT playlist_tracks_position_key,
    ADD CONSTRAINT playlist_tracks_position_key UNIQUE (playlist_id, position);
