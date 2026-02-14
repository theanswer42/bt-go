-- Drop tables in reverse order to respect foreign key constraints
DROP INDEX IF EXISTS idx_file_snapshots_created;
DROP INDEX IF EXISTS idx_file_snapshots_content;
DROP INDEX IF EXISTS idx_file_snapshots_file;
DROP TABLE IF EXISTS file_snapshots;

DROP INDEX IF EXISTS idx_files_deleted;
DROP INDEX IF EXISTS idx_files_directory;
DROP TABLE IF EXISTS files;

DROP INDEX IF EXISTS idx_directories_path;
DROP TABLE IF EXISTS directories;

DROP TABLE IF EXISTS contents;
