-- SQL queries for bt database operations
-- sqlc will generate type-safe Go code from these queries
-- See: https://docs.sqlc.dev/en/latest/

-- Directory queries

-- name: GetDirectoryByPath :one
SELECT * FROM directories WHERE path = ? LIMIT 1;

-- name: GetDirectoriesByPathPrefix :many
SELECT * FROM directories WHERE path LIKE ?1 ORDER BY path;

-- name: InsertDirectory :one
INSERT INTO directories (id, path, created_at, encrypted)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: DeleteDirectoryByID :exec
DELETE FROM directories WHERE id = ?;

-- File queries

-- name: GetFileByID :one
SELECT * FROM files WHERE id = ? LIMIT 1;

-- name: GetFilesByDirectoryID :many
SELECT * FROM files WHERE directory_id = ?;

-- name: GetFileByDirectoryAndName :one
SELECT * FROM files WHERE directory_id = ? AND name = ? LIMIT 1;

-- name: UpdateFileDirectoryAndName :exec
UPDATE files SET directory_id = ?, name = ? WHERE id = ?;

-- name: InsertFile :one
INSERT INTO files (id, name, directory_id, current_snapshot_id, deleted)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateFileCurrentSnapshot :exec
UPDATE files SET current_snapshot_id = ? WHERE id = ?;

-- FileSnapshot queries

-- name: GetFileSnapshotByID :one
SELECT * FROM file_snapshots WHERE id = ? LIMIT 1;

-- name: GetFileSnapshotsByFileID :many
SELECT * FROM file_snapshots WHERE file_id = ? ORDER BY created_at;

-- name: GetFileSnapshotByFileAndContent :one
SELECT * FROM file_snapshots WHERE file_id = ? AND content_id = ? LIMIT 1;

-- name: InsertFileSnapshot :one
INSERT INTO file_snapshots (id, file_id, content_id, created_at, size, permissions, uid, gid, accessed_at, modified_at, changed_at, born_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- Content queries

-- name: GetContentByID :one
SELECT * FROM contents WHERE id = ? LIMIT 1;

-- name: InsertContent :one
INSERT INTO contents (id, created_at, encrypted_content_id)
VALUES (?, ?, ?)
RETURNING *;

-- Backup operation queries

-- name: InsertBackupOperation :one
INSERT INTO backup_operations (started_at, operation, parameters)
VALUES (?, ?, ?)
RETURNING *;

-- name: UpdateBackupOperationFinished :exec
UPDATE backup_operations SET finished_at = ?, status = ? WHERE id = ?;

-- name: GetMaxBackupOperationID :one
SELECT CAST(COALESCE(MAX(id), 0) AS INTEGER) AS max_id FROM backup_operations;

-- name: GetBackupOperations :many
SELECT * FROM backup_operations ORDER BY id DESC LIMIT ?;
