-- +goose Up
-- One row per follow edge (specs/services/social.md). target_id references a
-- User Profile or Catalog artist by ID only: no cross-schema foreign keys.
CREATE TABLE social.follows (
    follower_id uuid        NOT NULL,
    target_type text        NOT NULL CHECK (target_type IN ('user', 'artist')),
    target_id   uuid        NOT NULL,
    followed_at timestamptz NOT NULL,
    PRIMARY KEY (follower_id, target_type, target_id),
    CHECK (target_type <> 'user' OR follower_id <> target_id)
);

-- "Following" pages: newest first per follower and target type.
CREATE INDEX follows_following_idx ON social.follows (follower_id, target_type, followed_at DESC, target_id DESC);
-- "Followers" pages: newest first per target.
CREATE INDEX follows_followers_idx ON social.follows (target_type, target_id, followed_at DESC, follower_id DESC);

-- Denormalised counters, maintained in the statement that changes an edge, so
-- profile and artist headers never count(*) a large follower set.
CREATE TABLE social.counters (
    subject_type      text   NOT NULL CHECK (subject_type IN ('user', 'artist')),
    subject_id        uuid   NOT NULL,
    followers         bigint NOT NULL DEFAULT 0 CHECK (followers >= 0),
    following_users   bigint NOT NULL DEFAULT 0 CHECK (following_users >= 0),
    following_artists bigint NOT NULL DEFAULT 0 CHECK (following_artists >= 0),
    PRIMARY KEY (subject_type, subject_id)
);

-- +goose Down
DROP TABLE social.counters;
DROP TABLE social.follows;
