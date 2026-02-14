-- SQL queries for bt database operations
-- sqlc will generate type-safe Go code from these queries
-- See: https://docs.sqlc.dev/en/latest/

-- Directory queries

-- name: GetDirectory :one
SELECT * FROM directories WHERE id = ? LIMIT 1;

-- name: ListDirectories :many
SELECT * FROM directories ORDER BY path;

-- name: CreateDirectory :one
INSERT INTO directories (id, path, created_at)
VALUES (?, ?, ?)
RETURNING *;
