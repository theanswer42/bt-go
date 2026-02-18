package staging

import (
	"fmt"
	"io/fs"

	"bt-go/internal/bt"
	"bt-go/internal/database/sqlc"
)

// stagedOperation represents a file staged for backup.
// This is an internal type - external code interacts with staged
// operations through the ProcessNext callback pattern.
//
// The operation stores directory ID + relative path to identify the file,
// rather than a database File ID. This avoids writing to the metadata
// store during staging.
type stagedOperation struct {
	DirectoryID  string            `json:"directory_id"`
	RelativePath string            `json:"relative_path"`
	Snapshot     sqlc.FileSnapshot `json:"snapshot"`
}

// validateStatUnchanged checks that file metadata hasn't changed.
// We ignore access time as it may change from our read.
func validateStatUnchanged(info1, info2 fs.FileInfo, stat1, stat2 *bt.StatData) error {
	if info1.Size() != info2.Size() {
		return fmt.Errorf("size changed: %d -> %d", info1.Size(), info2.Size())
	}
	if info1.Mode() != info2.Mode() {
		return fmt.Errorf("mode changed: %v -> %v", info1.Mode(), info2.Mode())
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		return fmt.Errorf("mtime changed: %v -> %v", info1.ModTime(), info2.ModTime())
	}
	if !stat1.Ctime.Equal(stat2.Ctime) {
		return fmt.Errorf("ctime changed: %v -> %v", stat1.Ctime, stat2.Ctime)
	}
	if stat1.UID != stat2.UID {
		return fmt.Errorf("uid changed: %d -> %d", stat1.UID, stat2.UID)
	}
	if stat1.GID != stat2.GID {
		return fmt.Errorf("gid changed: %d -> %d", stat1.GID, stat2.GID)
	}
	return nil
}
