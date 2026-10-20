-- name: CreateGuest :one
INSERT INTO guests (token_hash, display_name)
VALUES ($1, $2)
RETURNING *;

-- name: GuestByTokenHash :one
SELECT * FROM guests
WHERE token_hash = $1;

-- name: TouchGuest :exec
UPDATE guests SET last_seen_at = now()
WHERE id = $1;

-- name: DeleteGuestsIdleBefore :execrows
DELETE FROM guests
WHERE last_seen_at < $1;
