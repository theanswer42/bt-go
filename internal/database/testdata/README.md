# Test fixtures

## `mattn_v1.14.db`

A bt database written by the **previous** SQLite driver,
`github.com/mattn/go-sqlite3` v1.14.53 (CGO), used by
`TestSQLiteDatabase_ReadsLegacyMattnDatabase` to prove databases created by
older bt releases still read correctly with `modernc.org/sqlite`.

It was produced once, before the driver switch, by a throwaway program that
ran all migrations (schema version 3) and then wrote rows through the real
`SQLiteDatabase` methods with fixed clock values:

- `directories`: `/home/user/docs` (id `dir-1`)
- `files`: `a.txt`, `b.txt`, each with one snapshot (`snap-a`, `snap-b`)
- `contents`: `content-a`, `content-b`
- `backup_operations`: one finished (`BackupAll`), one still running (`AddDirectory`)

Timestamps use UTC, -08:00 and +05:30 offsets, whole seconds and various
fractional lengths down to 1ns, plus NULL `born_at` and `finished_at`. mattn
stored them as text in the layout `2006-01-02 15:04:05.999999999-07:00`, e.g.
`2025-03-14 09:26:53.589793238-08:00`. The expected values are spelled out in
the test.

It cannot be regenerated now that the CGO driver is gone; do not modify it.
