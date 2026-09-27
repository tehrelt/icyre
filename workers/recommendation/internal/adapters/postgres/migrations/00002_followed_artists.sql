-- +goose Up
-- social.followed / social.unfollowed for artist targets (Social Service,
-- EPIC-015); same last-writer-wins tombstones as the likes.
CREATE TABLE recommendation.followed_artists (
    user_id    uuid        NOT NULL,
    artist_id  uuid        NOT NULL,
    followed   boolean     NOT NULL,
    changed_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, artist_id)
);

-- +goose Down
DROP TABLE recommendation.followed_artists;
