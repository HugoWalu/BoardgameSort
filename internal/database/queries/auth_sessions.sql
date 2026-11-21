-- name: CreateAuthSession :one
INSERT INTO auth_sessions (user_id, token_hash)
VALUES ($1, $2)
RETURNING id, user_id, token_hash, created_at, last_seen_at, revoked_at;
