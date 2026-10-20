-- +goose Up
-- +goose StatementBegin
-- Guests: identity = (opaque cookie token ↔ guestId in DB).
-- Only the SHA-256 of the token is stored, never the token itself.
CREATE TABLE guests (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash   bytea NOT NULL UNIQUE,
    display_name text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now()
);
-- +goose StatementEnd

CREATE INDEX guests_last_seen_idx ON guests (last_seen_at);
