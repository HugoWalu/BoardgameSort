-- name: CreateGame :one
INSERT INTO games (
    title,
    year,
    bgg_id,
    min_players,
    max_players,
    best_players,
    recommended_players,
    playing_time,
    min_time,
    max_time,
    weight,
    image_url,
    categories,
    mechanics,
    teach_override_min,
    source
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
)
RETURNING
    id,
    title,
    year,
    bgg_id,
    min_players,
    max_players,
    best_players,
    recommended_players,
    playing_time,
    min_time,
    max_time,
    weight,
    image_url,
    categories,
    mechanics,
    teach_override_min,
    source,
    updated_at;
