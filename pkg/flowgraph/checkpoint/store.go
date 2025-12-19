// Package checkpoint provides persistent checkpoint storage for crash recovery.
package checkpoint

import (
	"errors"
	"time"
)

// Store persists checkpoints for crash recovery.
// Implementations must be safe for concurrent use.
type Store interface {
	// Save stores a checkpoint for a run at a specific node.
	// Overwrites if checkpoint for (runID, nodeID) already exists.
	Save(runID, nodeID string, data []byte) error

	// Load retrieves a checkpoint.
	// Returns ErrNotFound if checkpoint doesn't exist.
	Load(runID, nodeID string) ([]byte, error)

	// List returns all checkpoints for a run, ordered by sequence.
	// Returns empty slice (not error) if run has no checkpoints.
	List(runID string) ([]Info, error)

	// Delete removes a specific checkpoint.
	// Returns nil if checkpoint doesn't exist.
	Delete(runID, nodeID string) error

	// DeleteRun removes all checkpoints for a run.
	// Returns nil if run has no checkpoints.
	DeleteRun(runID string) error

	// Close releases any resources (connections, files).
	Close() error
}

// Info provides metadata without loading full state.
type Info struct {
	RunID     string
	NodeID    string
	Sequence  int
	Timestamp time.Time
	Size      int64
}

// Sentinel errors for checkpoint operations.
var (
	// ErrNotFound indicates a checkpoint doesn't exist.
	ErrNotFound = errors.New("checkpoint not found")

	// ErrStoreClosed indicates the store has been closed.
	ErrStoreClosed = errors.New("checkpoint store closed")
)
