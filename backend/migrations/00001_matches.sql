-- +goose Up
-- Матчи гостевого MVP. Аккаунты, Elo и рейтинговые поля добавляются
-- отдельными миграциями на соответствующих этапах (B17, B21).

CREATE TABLE matches (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    mode        text NOT NULL DEFAULT 'private', -- private | ranked
    status      text NOT NULL DEFAULT 'active',  -- active | finished | aborted
    winner_side text,                            -- a | b | NULL (aborted/ничья)
    end_reason  text,                            -- team_wiped | surrender | timeout_streak | forfeit | server_restart
    created_at  timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    CHECK (mode IN ('private', 'ranked')),
    CHECK (status IN ('active', 'finished', 'aborted')),
    CHECK (winner_side IN ('a', 'b')),
    CHECK (status != 'finished' OR (winner_side IS NOT NULL AND end_reason IS NOT NULL AND finished_at IS NOT NULL))
);

CREATE TABLE match_participants (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id     uuid NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    side         text NOT NULL,                 -- a | b
    display_name text NOT NULL,
    guest_id     text NOT NULL,                 -- серверный идентификатор гостя (B08), не ник
    CHECK (side IN ('a', 'b')),
    UNIQUE (match_id, side)
);

CREATE INDEX idx_matches_created_at ON matches (created_at DESC);

-- +goose Down
DROP TABLE match_participants;
DROP TABLE matches;
