package staging

import "io"

// stagingStore abstracts the storage mechanics for a staging area.
// Implementations handle content storage and operation queue management.
// Concurrency is managed by the caller (stagingArea.mu), so stores
// do not need to be safe for concurrent use.
type stagingStore interface {
	// StoreContent reads from r, computes SHA-256, and stores content.
	// Deduplicates if checksum already exists. Returns checksum and size.
	StoreContent(r io.Reader) (checksum string, size int64, err error)

	// RemoveContent removes stored content by checksum (best-effort).
	RemoveContent(checksum string)

	// OpenContent returns a reader for stored content by checksum.
	OpenContent(checksum string) (io.ReadCloser, error)

	// ContentSize returns total bytes of all stored content.
	ContentSize() (int64, error)

	// Append adds an operation to the end of the queue.
	Append(op *stagedOperation) error

	// Peek returns the first operation in the queue without removing it.
	// Returns nil if the queue is empty.
	Peek() (*stagedOperation, error)

	// Pop removes the first operation matching directoryID, relativePath, and checksum.
	// Returns the number of remaining operations referencing the same checksum
	// (so the caller can decide whether to call RemoveContent).
	Pop(directoryID, relativePath, checksum string) (checksumRefsRemaining int, err error)

	// Len returns the number of operations in the queue.
	Len() (int, error)

	// Contains reports whether an operation with the given directoryID and
	// relativePath exists in the queue.
	Contains(directoryID, relativePath string) (bool, error)
}
