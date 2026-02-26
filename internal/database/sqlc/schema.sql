-- This file is auto-generated from migration files.
-- DO NOT EDIT MANUALLY. Run 'make generate-schema' to regenerate.
-- Source: internal/database/migrations/files/*.sql

CREATE TABLE backup_operations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    operation TEXT NOT NULL,
    parameters TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'running'
);

CREATE TABLE contents (
    id TEXT PRIMARY KEY,  -- SHA-256 checksum (not a UUID)
    created_at DATETIME NOT NULL
, encrypted_content_id TEXT REFERENCES contents(id));

CREATE TABLE directories (
    id TEXT PRIMARY KEY,  -- UUID
    path TEXT NOT NULL UNIQUE,  -- Absolute path on host
    created_at DATETIME NOT NULL
, encrypted INTEGER NOT NULL DEFAULT 0);

CREATE TABLE file_snapshots (
    id TEXT PRIMARY KEY,  -- UUID
    file_id TEXT NOT NULL,
    content_id TEXT NOT NULL,  -- Checksum (foreign key to contents)
    created_at DATETIME NOT NULL,
    size INTEGER NOT NULL,
    permissions INTEGER NOT NULL,  -- File mode/permissions
    uid INTEGER NOT NULL,
    gid INTEGER NOT NULL,
    accessed_at DATETIME NOT NULL,  -- atime
    modified_at DATETIME NOT NULL,  -- mtime
    changed_at DATETIME NOT NULL,   -- ctime
    born_at DATETIME,  -- birthtime (nullable, not available on all filesystems)
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE,
    FOREIGN KEY (content_id) REFERENCES contents(id) ON DELETE RESTRICT
);

CREATE TABLE files (
    id TEXT PRIMARY KEY,  -- UUID
    name TEXT NOT NULL,  -- Relative path within directory
    directory_id TEXT NOT NULL,
    current_snapshot_id TEXT,  -- Can be NULL for new files not yet snapshotted
    deleted BOOLEAN NOT NULL DEFAULT 0,
    FOREIGN KEY (directory_id) REFERENCES directories(id) ON DELETE CASCADE,
    FOREIGN KEY (current_snapshot_id) REFERENCES file_snapshots(id) ON DELETE SET NULL,
    UNIQUE(directory_id, name)  -- File name must be unique within a directory
);

CREATE INDEX idx_directories_path ON directories(path);

CREATE INDEX idx_file_snapshots_content ON file_snapshots(content_id);

CREATE INDEX idx_file_snapshots_created ON file_snapshots(created_at);

CREATE INDEX idx_file_snapshots_file ON file_snapshots(file_id);

CREATE INDEX idx_files_deleted ON files(deleted);

CREATE INDEX idx_files_directory ON files(directory_id);

