-- SQL queries for bt database operations
-- sqlc will generate type-safe Go code from these queries
-- See: https://docs.sqlc.dev/en/latest/

-- Directory queries

-- name: GetDirectoryByPath :one
SELECT * FROM directories WHERE path = ? LIMIT 1;

-- name: GetDirectoriesByPathPrefix :many
SELECT * FROM directories WHERE path LIKE ?1 ORDER BY path;

-- name: InsertDirectory :one
INSERT INTO directories (id, path, created_at)
VALUES (?, ?, ?)
RETURNING *;

-- name: DeleteDirectoryByID :exec
DELETE FROM directories WHERE id = ?;

-- File queries

-- name: GetFilesByDirectoryID :many
SELECT * FROM files WHERE directory_id = ?;

-- name: UpdateFileDirectoryAndName :exec
UPDATE files SET directory_id = ?, name = ? WHERE id = ?;

-- name: InsertFile :one
INSERT INTO files (id, name, directory_id, current_snapshot_id, deleted)
VALUES (?, ?, ?, ?, ?)
RETURNING *;
