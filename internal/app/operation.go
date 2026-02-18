package app

// BackupOperation tracks a CLI operation that may mutate the database.
// Operations are created in memory with ID=0. Only DB-mutating commands
// persist them (giving them an auto-increment ID from the database).
type BackupOperation struct {
	ID         int64
	Operation  string
	Parameters string
	Status     string // "success" or "error"
}

// NewBackupOperation creates a new in-memory backup operation.
func NewBackupOperation(operation, parameters string) *BackupOperation {
	return &BackupOperation{
		Operation:  operation,
		Parameters: parameters,
		Status:     "success",
	}
}

// Persisted returns true if this operation has been saved to the database.
func (op *BackupOperation) Persisted() bool {
	return op.ID != 0
}
