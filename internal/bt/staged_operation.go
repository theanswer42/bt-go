package bt

import "bt-go/internal/database/sqlc"

// StagedOperation represents a file staged for backup.
// It contains all the information needed to complete the backup:
// - Which directory it belongs to (DirectoryID)
// - The file snapshot data including FileID, ContentID (checksum), and stat metadata
type StagedOperation struct {
	DirectoryID string
	Snapshot    sqlc.FileSnapshot
}
