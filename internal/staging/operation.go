package staging

import "bt-go/internal/database/sqlc"

// stagedOperation represents a file staged for backup.
// This is an internal type - external code interacts with staged
// operations through the ProcessNext callback pattern.
type stagedOperation struct {
	DirectoryID string            `json:"directory_id"`
	Snapshot    sqlc.FileSnapshot `json:"snapshot"`
}
