-- Contents table: content-addressable storage
-- ID is the SHA-256 checksum of the content itself
CREATE TABLE contents (
    id TEXT PRIMARY KEY,  -- SHA-256 checksum (not a UUID)
    created_at DATETIME NOT NULL
);

-- Directories table: tracked directories on the local host
CREATE TABLE directories (
    id TEXT PRIMARY KEY,  -- UUID
    path TEXT NOT NULL UNIQUE,  -- Absolute path on host
    created_at DATETIME NOT NULL
);

-- Create index for path prefix searches
CREATE INDEX idx_directories_path ON directories(path);

-- Files table: files within tracked directories
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

-- Create indexes for common queries
CREATE INDEX idx_files_directory ON files(directory_id);
CREATE INDEX idx_files_deleted ON files(deleted);

-- FileSnapshots table: state of a file at a specific point in time
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

-- Create indexes for common queries
CREATE INDEX idx_file_snapshots_file ON file_snapshots(file_id);
CREATE INDEX idx_file_snapshots_content ON file_snapshots(content_id);
CREATE INDEX idx_file_snapshots_created ON file_snapshots(created_at);
