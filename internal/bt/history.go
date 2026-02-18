package bt

import (
	"fmt"

	"bt-go/internal/database/sqlc"
)

// GetHistory returns the most recent backup operations, ordered newest first.
func (s *BTService) GetHistory(limit int) ([]*sqlc.BackupOperation, error) {
	ops, err := s.database.ListBackupOperations(limit)
	if err != nil {
		return nil, fmt.Errorf("listing backup operations: %w", err)
	}
	return ops, nil
}
