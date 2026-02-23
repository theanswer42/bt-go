package bt

import (
	"time"

	"github.com/google/uuid"
)

// Clock abstracts time retrieval so business logic is deterministic in tests.
type Clock interface {
	Now() time.Time
}

// RealClock returns the actual current time.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

// IDGenerator abstracts unique ID generation so tests are deterministic.
type IDGenerator interface {
	New() string
}

// UUIDGenerator produces random UUIDs.
type UUIDGenerator struct{}

func (UUIDGenerator) New() string { return uuid.New().String() }
