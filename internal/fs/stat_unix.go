//go:build unix

package fs

import (
	"database/sql"
	"fmt"
	"io/fs"
	"syscall"
	"time"

	"bt-go/internal/bt"
)

// ExtractStatData extracts Unix-specific stat data from a FileInfo.
func (m *OSFilesystemManager) ExtractStatData(info fs.FileInfo) (*bt.StatData, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("cannot extract stat data: expected *syscall.Stat_t, got %T", info.Sys())
	}

	return &bt.StatData{
		UID:   int64(stat.Uid),
		GID:   int64(stat.Gid),
		Atime: time.Unix(stat.Atim.Sec, stat.Atim.Nsec),
		Ctime: time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec),
		// Birth time is not available on most Unix filesystems
		BirthTime: sql.NullTime{Valid: false},
	}, nil
}
