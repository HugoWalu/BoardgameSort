-- name: CreateCopy :one
INSERT INTO copies (game_id, owner_id, status, lent_to, lent_since, location)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, game_id, owner_id, status, lent_to, lent_since, location;
