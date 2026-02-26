-- SQLite does not support DROP COLUMN before 3.35.0.
-- Recreate tables without encryption columns.

-- Recreate contents without encrypted_content_id
CREATE TABLE contents_new (
    id TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL
);
INSERT INTO contents_new SELECT id, created_at FROM contents;
DROP TABLE contents;
ALTER TABLE contents_new RENAME TO contents;

-- Recreate directories without encrypted
CREATE TABLE directories_new (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL
);
INSERT INTO directories_new SELECT id, path, created_at FROM directories;
DROP TABLE directories;
ALTER TABLE directories_new RENAME TO directories;

CREATE INDEX idx_directories_path ON directories(path);
