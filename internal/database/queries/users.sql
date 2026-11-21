-- name: CreateUser :one
INSERT INTO users (name)
VALUES ($1)
RETURNING id, name, created_at;
