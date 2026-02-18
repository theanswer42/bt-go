CREATE TABLE backup_operations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    operation TEXT NOT NULL,
    parameters TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'running'
);
