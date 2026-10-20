-- name: CreateMatch :one
INSERT INTO matches (mode)
VALUES ($1)
RETURNING id, mode, status, winner_side, end_reason, created_at, finished_at;

-- name: CreateParticipant :one
INSERT INTO match_participants (match_id, side, display_name, guest_id)
VALUES ($1, $2, $3, $4)
RETURNING id, match_id, side, display_name, guest_id;

-- name: GetMatch :one
SELECT id, mode, status, winner_side, end_reason, created_at, finished_at
FROM matches
WHERE id = $1;

-- name: ListParticipantsByMatch :many
SELECT id, match_id, side, display_name, guest_id
FROM match_participants
WHERE match_id = $1
ORDER BY side;
