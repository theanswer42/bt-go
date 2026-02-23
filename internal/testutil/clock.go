package testutil

import (
	"fmt"
	"sync"
	"time"
)

// StubClock returns a fixed time. Safe for concurrent use.
type StubClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewStubClock creates a StubClock set to the given time.
func NewStubClock(t time.Time) *StubClock {
	return &StubClock{now: t}
}

// FixedClock returns a StubClock set to 2024-01-15 10:30:00 UTC.
func FixedClock() *StubClock {
	return NewStubClock(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))
}

func (c *StubClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward by d.
func (c *StubClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// StubIDGenerator returns sequential IDs: "id-1", "id-2", etc.
type StubIDGenerator struct {
	mu      sync.Mutex
	counter int
}

func NewStubIDGenerator() *StubIDGenerator {
	return &StubIDGenerator{}
}

func (g *StubIDGenerator) New() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	return fmt.Sprintf("id-%d", g.counter)
}
