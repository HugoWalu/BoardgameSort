-- name: CreateVeto :one
INSERT INTO vetoes (user_id, game_id)
VALUES ($1, $2)
RETURNING user_id, game_id, created_at;
