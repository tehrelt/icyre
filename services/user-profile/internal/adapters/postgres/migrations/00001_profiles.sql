-- +goose Up
CREATE TABLE profile.profiles (
    user_id      uuid        PRIMARY KEY, -- account ID issued by Auth (no cross-schema FK)
    username     text        NOT NULL CHECK (username ~ '^[a-z0-9][a-z0-9._]{1,28}[a-z0-9]$'),
    display_name text        NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 50),
    avatar_key   text        NOT NULL DEFAULT '',
    bio          text        NOT NULL DEFAULT '' CHECK (char_length(bio) <= 300),
    country      text        NOT NULL DEFAULT '' CHECK (country = '' OR country ~ '^[A-Z]{2}$'),
    language     text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    CONSTRAINT profiles_username_key UNIQUE (username)
);

-- +goose Down
DROP TABLE profile.profiles;
