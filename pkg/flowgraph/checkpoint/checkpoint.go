package checkpoint

import (
	"encoding/json"
	"time"
)

// Version is the current checkpoint format version.
// Increment when making breaking changes to checkpoint structure.
const Version = 1

// Checkpoint is the persisted snapshot of execution state.
// It contains all information needed to resume execution.
type Checkpoint struct {
	// Metadata
	Version   int       `json:"version"`
	RunID     string    `json:"run_id"`
	NodeID    string    `json:"node_id"`
	Sequence  int       `json:"sequence"`
	Timestamp time.Time `json:"timestamp"`

	// Execution state
	State    json.RawMessage `json:"state"`
	NextNode string          `json:"next_node"`

	// Execution context
	Attempt    int    `json:"attempt"`
	PrevNodeID string `json:"prev_node_id,omitempty"`
}

// Marshal serializes a checkpoint to JSON.
func (c *Checkpoint) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

// Unmarshal deserializes a checkpoint from JSON.
func Unmarshal(data []byte) (*Checkpoint, error) {
	var c Checkpoint
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// New creates a new checkpoint with the given parameters.
// State must already be JSON-serialized.
func New(runID, nodeID string, sequence int, state []byte, nextNode string) *Checkpoint {
	return &Checkpoint{
		Version:   Version,
		RunID:     runID,
		NodeID:    nodeID,
		Sequence:  sequence,
		Timestamp: time.Now().UTC(),
		State:     state,
		NextNode:  nextNode,
		Attempt:   1,
	}
}

// WithAttempt sets the attempt number for retry tracking.
func (c *Checkpoint) WithAttempt(attempt int) *Checkpoint {
	c.Attempt = attempt
	return c
}

// WithPrevNode sets the previous node ID for debugging.
func (c *Checkpoint) WithPrevNode(prevNodeID string) *Checkpoint {
	c.PrevNodeID = prevNodeID
	return c
}
