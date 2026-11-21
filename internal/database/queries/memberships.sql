-- name: CreateMembership :one
INSERT INTO memberships (group_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING group_id, user_id, role, created_at;
