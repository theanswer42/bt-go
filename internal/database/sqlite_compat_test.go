package database

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bt-go/internal/database/migrations"
	"bt-go/internal/database/sqlc"
)

// legacyTimeLayout is the layout mattn/go-sqlite3 used to write time.Time
// values (its SQLiteTimestampFormats[0]). New writes must use the same layout.
const legacyTimeLayout = "2006-01-02 15:04:05.999999999-07:00"

// legacyFixture is a database written by bt with the mattn/go-sqlite3 driver.
// See testdata/README.md for how it was produced and what it contains.
const legacyFixture = "testdata/mattn_v1.14.db"

var (
	zonePST = time.FixedZone("PST", -8*3600)
	zoneIST = time.FixedZone("IST", 5*3600+1800)
)

// copyFixture copies a testdata database into a temp dir so tests never
// modify the checked-in file.
func copyFixture(t *testing.T, src string) string {
	t.Helper()

	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer in.Close()

	dst := filepath.Join(t.TempDir(), filepath.Base(src))
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("creating fixture copy: %v", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		t.Fatalf("copying fixture: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("closing fixture copy: %v", err)
	}
	return dst
}

// rawText returns the stored text of a column. Queries concatenate the column
// with an empty string: the resulting expression has no declared type, so the
// driver returns the raw text instead of converting DATETIME to time.Time.
func rawText(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()

	var s string
	if err := db.QueryRow(query, args...).Scan(&s); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return s
}

// timeCheck is one timestamp column value to verify.
type timeCheck struct {
	column string
	got    sql.NullTime
	want   sql.NullTime
}

func checkTimes(t *testing.T, checks []timeCheck) {
	t.Helper()

	for _, c := range checks {
		t.Run(c.column, func(t *testing.T) {
			if c.got.Valid != c.want.Valid {
				t.Fatalf("Valid = %v, want %v", c.got.Valid, c.want.Valid)
			}
			if !c.got.Time.Equal(c.want.Time) {
				t.Errorf("time = %v, want %v", c.got.Time, c.want.Time)
			}
			_, gotOff := c.got.Time.Zone()
			_, wantOff := c.want.Time.Zone()
			if gotOff != wantOff {
				t.Errorf("UTC offset = %ds, want %ds", gotOff, wantOff)
			}
		})
	}
}

func valid(tm time.Time) sql.NullTime { return sql.NullTime{Time: tm, Valid: true} }

// TestSQLiteDatabase_ReadsLegacyMattnDatabase verifies that a database written
// by the previous CGO driver (mattn/go-sqlite3) opens, passes the migration
// check, and that every timestamp column scans back into the same instant.
func TestSQLiteDatabase_ReadsLegacyMattnDatabase(t *testing.T) {
	db, err := NewSQLiteDatabase(copyFixture(t, legacyFixture), nil, nil)
	if err != nil {
		t.Fatalf("NewSQLiteDatabase() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.CheckMigrations(); err != nil {
		t.Fatalf("CheckMigrations() error = %v", err)
	}

	dir, err := db.FindDirectoryByPath("/home/user/docs")
	if err != nil || dir == nil {
		t.Fatalf("FindDirectoryByPath() = %v, %v", dir, err)
	}
	contentA, err := db.FindContentByChecksum("content-a")
	if err != nil || contentA == nil {
		t.Fatalf("FindContentByChecksum(content-a) = %v, %v", contentA, err)
	}
	contentB, err := db.FindContentByChecksum("content-b")
	if err != nil || contentB == nil {
		t.Fatalf("FindContentByChecksum(content-b) = %v, %v", contentB, err)
	}

	snapshot := func(name string) *sqlc.FileSnapshot {
		t.Helper()
		f, err := db.FindFileByPath(dir, name)
		if err != nil || f == nil {
			t.Fatalf("FindFileByPath(%s) = %v, %v", name, f, err)
		}
		snaps, err := db.FindFileSnapshotsForFile(f)
		if err != nil || len(snaps) != 1 {
			t.Fatalf("FindFileSnapshotsForFile(%s) = %v, %v", name, snaps, err)
		}
		return snaps[0]
	}
	snapA := snapshot("a.txt")
	snapB := snapshot("b.txt")

	ops, err := db.ListBackupOperations(10)
	if err != nil || len(ops) != 2 {
		t.Fatalf("ListBackupOperations() = %v, %v", ops, err)
	}
	// Listed newest first.
	op1, op2 := ops[1], ops[0]

	checkTimes(t, []timeCheck{
		{"directories.created_at", valid(dir.CreatedAt), valid(time.Date(2025, 3, 14, 9, 26, 53, 589793238, zonePST))},
		{"contents.created_at/a", valid(contentA.CreatedAt), valid(time.Date(2025, 3, 14, 9, 30, 0, 0, zonePST))},
		{"contents.created_at/b", valid(contentB.CreatedAt), valid(time.Date(2025, 3, 14, 17, 31, 0, 123000000, time.UTC))},
		{"file_snapshots.created_at/a", valid(snapA.CreatedAt), valid(time.Date(2025, 3, 14, 17, 30, 0, 111111111, time.UTC))},
		{"file_snapshots.accessed_at/a", valid(snapA.AccessedAt), valid(time.Date(2025, 3, 14, 22, 0, 1, 222222222, zoneIST))},
		{"file_snapshots.modified_at/a", valid(snapA.ModifiedAt), valid(time.Date(2025, 3, 14, 8, 0, 0, 0, zonePST))},
		{"file_snapshots.changed_at/a", valid(snapA.ChangedAt), valid(time.Date(2025, 3, 14, 8, 0, 0, 333333000, zonePST))},
		{"file_snapshots.born_at/a", snapA.BornAt, valid(time.Date(2024, 1, 1, 0, 0, 0, 444000000, time.UTC))},
		{"file_snapshots.created_at/b", valid(snapB.CreatedAt), valid(time.Date(2025, 3, 14, 9, 31, 0, 0, zonePST))},
		{"file_snapshots.accessed_at/b", valid(snapB.AccessedAt), valid(time.Date(2025, 3, 14, 9, 31, 0, 1, zonePST))},
		{"file_snapshots.modified_at/b", valid(snapB.ModifiedAt), valid(time.Date(2025, 3, 14, 17, 31, 0, 2, time.UTC))},
		{"file_snapshots.changed_at/b", valid(snapB.ChangedAt), valid(time.Date(2025, 3, 14, 23, 1, 0, 3, zoneIST))},
		{"file_snapshots.born_at/b (NULL)", snapB.BornAt, sql.NullTime{}},
		{"backup_operations.started_at/1", valid(op1.StartedAt), valid(time.Date(2025, 3, 15, 1, 2, 3, 4, zoneIST))},
		{"backup_operations.finished_at/1", op1.FinishedAt, valid(time.Date(2025, 3, 15, 1, 2, 9, 999999999, zoneIST))},
		{"backup_operations.started_at/2", valid(op2.StartedAt), valid(time.Date(2025, 12, 31, 23, 59, 59, 500000000, time.UTC))},
		{"backup_operations.finished_at/2 (NULL)", op2.FinishedAt, sql.NullTime{}},
	})

	// A row written by the new driver into the legacy DB uses the legacy format,
	// and an unchanged legacy snapshot is still recognized as unchanged (no
	// spurious new snapshot) after a round trip through the new driver.
	t.Run("new writes match legacy format", func(t *testing.T) {
		at := time.Date(2026, 1, 2, 3, 4, 5, 600000000, zonePST)
		if _, err := db.CreateContent("content-new", ""); err != nil {
			t.Fatalf("CreateContent() error = %v", err)
		}
		if _, err := db.db.Exec("UPDATE contents SET created_at = ? WHERE id = 'content-new'", at); err != nil {
			t.Fatalf("update: %v", err)
		}
		got := rawText(t, db.db, "SELECT created_at || '' FROM contents WHERE id = 'content-new'")
		if want := "2026-01-02 03:04:05.6-08:00"; got != want {
			t.Errorf("stored created_at = %q, want %q", got, want)
		}
	})

	t.Run("unchanged legacy snapshot is a no-op", func(t *testing.T) {
		same := *snapA
		same.ID = "snap-a-again"
		if err := db.CreateFileSnapshotAndContent(dir.ID, "a.txt", &same, ""); err != nil {
			t.Fatalf("CreateFileSnapshotAndContent() error = %v", err)
		}
		f, _ := db.FindFileByPath(dir, "a.txt")
		snaps, err := db.FindFileSnapshotsForFile(f)
		if err != nil {
			t.Fatalf("FindFileSnapshotsForFile() error = %v", err)
		}
		if len(snaps) != 1 {
			t.Errorf("got %d snapshots, want 1 (unchanged file must not create a snapshot)", len(snaps))
		}
	})
}

// TestSQLiteDatabase_TimestampRoundTrip writes every timestamp column through
// the Database methods, reopens the file, and checks both the scanned value and
// the stored text.
func TestSQLiteDatabase_TimestampRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		tm   time.Time
	}{
		{name: "utc with nanoseconds", tm: time.Date(2025, 6, 1, 12, 0, 0, 123456789, time.UTC)},
		{name: "negative offset, whole seconds", tm: time.Date(2025, 1, 15, 23, 59, 59, 0, zonePST)},
		{name: "half-hour positive offset", tm: time.Date(2025, 2, 28, 0, 0, 0, 1, zoneIST)},
		{name: "local zone with monotonic reading", tm: time.Now()},
		{name: "unix epoch", tm: time.Unix(0, 0).UTC()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "rt.db")
			conn, err := OpenConnection(path)
			if err != nil {
				t.Fatalf("OpenConnection() error = %v", err)
			}
			if err := migrations.MigrateUp(conn); err != nil {
				t.Fatalf("MigrateUp() error = %v", err)
			}
			nowFn := func() time.Time { return tt.tm }
			db := NewSQLiteDatabaseFromDB(conn, nowFn, nil)

			dir, err := db.CreateDirectory("/data", false)
			if err != nil {
				t.Fatalf("CreateDirectory() error = %v", err)
			}
			if _, err := db.CreateContent("c1", ""); err != nil {
				t.Fatalf("CreateContent() error = %v", err)
			}
			file, err := db.FindOrCreateFile(dir, "f")
			if err != nil {
				t.Fatalf("FindOrCreateFile() error = %v", err)
			}
			err = db.CreateFileSnapshot(&sqlc.FileSnapshot{
				ID: "s1", FileID: file.ID, ContentID: "c1",
				CreatedAt: tt.tm, AccessedAt: tt.tm, ModifiedAt: tt.tm, ChangedAt: tt.tm,
				BornAt: valid(tt.tm),
			})
			if err != nil {
				t.Fatalf("CreateFileSnapshot() error = %v", err)
			}
			op, err := db.CreateBackupOperation("BackupAll", "")
			if err != nil {
				t.Fatalf("CreateBackupOperation() error = %v", err)
			}
			if err := db.FinishBackupOperation(op.ID, "success"); err != nil {
				t.Fatalf("FinishBackupOperation() error = %v", err)
			}
			if err := db.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}

			db, err = NewSQLiteDatabase(path, nil, nil)
			if err != nil {
				t.Fatalf("reopening: %v", err)
			}
			t.Cleanup(func() { db.Close() })

			// Stored text: identical to what mattn/go-sqlite3 wrote, so old and
			// new rows share one format (and no monotonic "m=" suffix leaks in).
			wantText := tt.tm.Format(legacyTimeLayout)
			for _, q := range []string{
				"SELECT created_at || '' FROM directories",
				"SELECT created_at || '' FROM contents",
				"SELECT created_at || '' FROM file_snapshots",
				"SELECT accessed_at || '' FROM file_snapshots",
				"SELECT modified_at || '' FROM file_snapshots",
				"SELECT changed_at || '' FROM file_snapshots",
				"SELECT born_at || '' FROM file_snapshots",
				"SELECT started_at || '' FROM backup_operations",
				"SELECT finished_at || '' FROM backup_operations",
			} {
				if got := rawText(t, db.db, q); got != wantText {
					t.Errorf("%s = %q, want %q", q, got, wantText)
				}
			}

			gotDir, err := db.FindDirectoryByPath("/data")
			if err != nil || gotDir == nil {
				t.Fatalf("FindDirectoryByPath() = %v, %v", gotDir, err)
			}
			gotContent, err := db.FindContentByChecksum("c1")
			if err != nil || gotContent == nil {
				t.Fatalf("FindContentByChecksum() = %v, %v", gotContent, err)
			}
			gotFile, err := db.FindFileByPath(gotDir, "f")
			if err != nil || gotFile == nil {
				t.Fatalf("FindFileByPath() = %v, %v", gotFile, err)
			}
			snaps, err := db.FindFileSnapshotsForFile(gotFile)
			if err != nil || len(snaps) != 1 {
				t.Fatalf("FindFileSnapshotsForFile() = %v, %v", snaps, err)
			}
			ops, err := db.ListBackupOperations(1)
			if err != nil || len(ops) != 1 {
				t.Fatalf("ListBackupOperations() = %v, %v", ops, err)
			}
			s, o := snaps[0], ops[0]

			want := valid(tt.tm)
			checkTimes(t, []timeCheck{
				{"directories.created_at", valid(gotDir.CreatedAt), want},
				{"contents.created_at", valid(gotContent.CreatedAt), want},
				{"file_snapshots.created_at", valid(s.CreatedAt), want},
				{"file_snapshots.accessed_at", valid(s.AccessedAt), want},
				{"file_snapshots.modified_at", valid(s.ModifiedAt), want},
				{"file_snapshots.changed_at", valid(s.ChangedAt), want},
				{"file_snapshots.born_at", s.BornAt, want},
				{"backup_operations.started_at", valid(o.StartedAt), want},
				{"backup_operations.finished_at", o.FinishedAt, want},
			})
		})
	}
}

// TestSQLiteDatabase_SnapshotOrderWithinOffset verifies that ORDER BY on a
// stored timestamp is chronological for times sharing a UTC offset, including
// mixed fractional-second lengths (the layout trims trailing zeros).
func TestSQLiteDatabase_SnapshotOrderWithinOffset(t *testing.T) {
	db := newTestDB(t)
	dir, err := db.CreateDirectory("/data", false)
	if err != nil {
		t.Fatalf("CreateDirectory() error = %v", err)
	}
	if _, err := db.CreateContent("c1", ""); err != nil {
		t.Fatalf("CreateContent() error = %v", err)
	}
	file, err := db.FindOrCreateFile(dir, "f")
	if err != nil {
		t.Fatalf("FindOrCreateFile() error = %v", err)
	}

	base := time.Date(2025, 5, 5, 10, 0, 0, 0, zonePST)
	// Inserted out of order; the expected chronological order is by index.
	ordered := []time.Time{
		base,
		base.Add(1),                      // .000000001
		base.Add(100 * time.Millisecond), // .1
		base.Add(120 * time.Millisecond), // .12
		base.Add(500 * time.Millisecond), // .5
		base.Add(time.Second),
		base.Add(10 * time.Hour),
	}
	insertOrder := []int{4, 0, 6, 2, 5, 1, 3}
	for _, i := range insertOrder {
		err := db.CreateFileSnapshot(&sqlc.FileSnapshot{
			ID: string(rune('a' + i)), FileID: file.ID, ContentID: "c1",
			CreatedAt: ordered[i], AccessedAt: base, ModifiedAt: base, ChangedAt: base,
		})
		if err != nil {
			t.Fatalf("CreateFileSnapshot() error = %v", err)
		}
	}

	snaps, err := db.FindFileSnapshotsForFile(file)
	if err != nil {
		t.Fatalf("FindFileSnapshotsForFile() error = %v", err)
	}
	if len(snaps) != len(ordered) {
		t.Fatalf("got %d snapshots, want %d", len(snaps), len(ordered))
	}
	for i, s := range snaps {
		if !s.CreatedAt.Equal(ordered[i]) {
			t.Errorf("snapshot[%d].CreatedAt = %v, want %v", i, s.CreatedAt, ordered[i])
		}
	}
}

// TestOpenConnection_PragmasOnEveryConnection verifies per-connection settings
// reach every connection in the pool, not just the first one.
func TestOpenConnection_PragmasOnEveryConnection(t *testing.T) {
	db, err := OpenConnection(filepath.Join(t.TempDir(), "pool.db"))
	if err != nil {
		t.Fatalf("OpenConnection() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Hold several connections at once so the pool must open new ones.
	const n = 4
	ctx := t.Context()
	conns := make([]*sql.Conn, 0, n)
	for i := 0; i < n; i++ {
		c, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("Conn() error = %v", err)
		}
		conns = append(conns, c)
	}
	if got := db.Stats().OpenConnections; got < n {
		t.Fatalf("OpenConnections = %d, want >= %d", got, n)
	}

	tests := []struct {
		pragma string
		want   int
	}{
		{pragma: "foreign_keys", want: 1},
		{pragma: "busy_timeout", want: 5000},
	}
	for i, c := range conns {
		for _, tt := range tests {
			var got int
			if err := c.QueryRowContext(ctx, "PRAGMA "+tt.pragma).Scan(&got); err != nil {
				t.Fatalf("conn %d: PRAGMA %s: %v", i, tt.pragma, err)
			}
			if got != tt.want {
				t.Errorf("conn %d: PRAGMA %s = %d, want %d", i, tt.pragma, got, tt.want)
			}
		}
	}
	for _, c := range conns {
		c.Close()
	}
}

// TestOpenConnection_MemoryDatabaseIsShared verifies that all queries against a
// ":memory:" database see the same data, even across transactions.
func TestOpenConnection_MemoryDatabaseIsShared(t *testing.T) {
	db, err := OpenConnection(":memory:")
	if err != nil {
		t.Fatalf("OpenConnection() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if got := db.Stats().MaxOpenConnections; got != 1 {
		t.Errorf("MaxOpenConnections = %d, want 1", got)
	}

	if _, err := db.Exec("CREATE TABLE t (v INTEGER)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if _, err := tx.Exec("INSERT INTO t VALUES (1)"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM t").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("count = %d, want 1", n)
	}
}

func TestOpenConnection_Errors(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{name: "path containing '?'", path: filepath.Join(t.TempDir(), "a?b.db"), wantErr: "must not contain '?'"},
		{name: "missing parent directory", path: filepath.Join(t.TempDir(), "missing", "x.db"), wantErr: "failed to connect"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := OpenConnection(tt.path)
			if err == nil {
				db.Close()
				t.Fatal("OpenConnection() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("OpenConnection() error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
