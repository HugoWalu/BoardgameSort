-- +goose Up
CREATE TABLE users (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE groups (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
    group_id   BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('host', 'member')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE invites (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    group_id   BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ
);

CREATE TABLE auth_sessions (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);

CREATE TABLE games (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title               TEXT NOT NULL,
    year                INTEGER,
    bgg_id              INTEGER UNIQUE,
    min_players         INTEGER,
    max_players         INTEGER,
    best_players        INTEGER[],
    recommended_players INTEGER[],
    playing_time        INTEGER,
    min_time            INTEGER,
    max_time            INTEGER,
    weight              NUMERIC(3,2),
    image_url           TEXT,
    categories          TEXT[],
    mechanics           TEXT[],
    teach_override_min  INTEGER,
    source              TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('bgg', 'manual')),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE copies (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    game_id    BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    owner_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status     TEXT NOT NULL DEFAULT 'owned' CHECK (status IN ('owned', 'lent', 'previously_owned')),
    lent_to    BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    lent_since TIMESTAMPTZ,
    location   TEXT,
    CHECK (status <> 'lent' OR lent_to IS NOT NULL)
);

CREATE TABLE vetoes (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id    BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, game_id)
);

CREATE INDEX copies_game_id_idx ON copies (game_id);
CREATE INDEX copies_lent_to_idx ON copies (lent_to) WHERE lent_to IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS vetoes;
DROP TABLE IF EXISTS copies;
DROP TABLE IF EXISTS games;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS invites;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS users;
