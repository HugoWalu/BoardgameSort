-- name: CreateInvite :one
INSERT INTO invites (group_id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, group_id, user_id, token_hash, expires_at, used_at;
