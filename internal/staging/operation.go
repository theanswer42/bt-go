package staging

import "bt-go/internal/database/sqlc"

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
