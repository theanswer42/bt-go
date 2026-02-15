//go:build unix

package staging

import (
	"database/sql"
	"fmt"
	"io/fs"
	"syscall"
	"time"
)

// statData holds platform-specific file metadata extracted from fs.FileInfo.
type statData struct {
	UID       int64
	GID       int64
	Atime     time.Time
	Ctime     time.Time
	BirthTime sql.NullTime
}

// extractStatData extracts Unix-specific stat data from a FileInfo.
// Returns an error if the underlying Sys() type is not *syscall.Stat_t,
// which would happen with mock filesystems that don't provide real stat data.
func extractStatData(info fs.FileInfo) (*statData, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("cannot extract stat data: expected *syscall.Stat_t, got %T", info.Sys())
	}

	return &statData{
		UID:   int64(stat.Uid),
		GID:   int64(stat.Gid),
		Atime: time.Unix(stat.Atim.Sec, stat.Atim.Nsec),
		Ctime: time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec),
		// Birth time is not available on most Unix filesystems
		BirthTime: sql.NullTime{Valid: false},
	}, nil
}
