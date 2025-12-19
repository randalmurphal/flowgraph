package checkpoint

import (
	"sort"
	"sync"
	"time"
)

// MemoryStore is an in-memory checkpoint store for testing.
// Data is lost when the process exits.
type MemoryStore struct {
	mu     sync.RWMutex
	data   map[string]map[string]storedCheckpoint // runID -> nodeID -> checkpoint
	maxSeq map[string]int                         // runID -> max sequence (O(1) lookup)
	closed bool
}

// storedCheckpoint holds checkpoint data with metadata for List().
type storedCheckpoint struct {
	data      []byte
	sequence  int
	timestamp time.Time
}

// NewMemoryStore creates a new in-memory checkpoint store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data:   make(map[string]map[string]storedCheckpoint),
		maxSeq: make(map[string]int),
	}
}

// Save implements Store.
func (m *MemoryStore) Save(runID, nodeID string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrStoreClosed
	}

	if m.data[runID] == nil {
		m.data[runID] = make(map[string]storedCheckpoint)
	}

	// Determine sequence number using O(1) lookup
	m.maxSeq[runID]++
	seq := m.maxSeq[runID]

	// Copy data to avoid retaining caller's slice
	stored := make([]byte, len(data))
	copy(stored, data)

	m.data[runID][nodeID] = storedCheckpoint{
		data:      stored,
		sequence:  seq,
		timestamp: time.Now().UTC(),
	}

	return nil
}

// Load implements Store.
func (m *MemoryStore) Load(runID, nodeID string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, ErrStoreClosed
	}

	run, ok := m.data[runID]
	if !ok {
		return nil, ErrNotFound
	}

	cp, ok := run[nodeID]
	if !ok {
		return nil, ErrNotFound
	}

	// Return a copy to prevent modification
	result := make([]byte, len(cp.data))
	copy(result, cp.data)
	return result, nil
}

// List implements Store.
func (m *MemoryStore) List(runID string) ([]Info, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, ErrStoreClosed
	}

	run, ok := m.data[runID]
	if !ok {
		return nil, nil
	}

	infos := make([]Info, 0, len(run))
	for nodeID, cp := range run {
		infos = append(infos, Info{
			RunID:     runID,
			NodeID:    nodeID,
			Sequence:  cp.sequence,
			Timestamp: cp.timestamp,
			Size:      int64(len(cp.data)),
		})
	}

	// Sort by sequence
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Sequence < infos[j].Sequence
	})

	return infos, nil
}

// Delete implements Store.
func (m *MemoryStore) Delete(runID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrStoreClosed
	}

	if run, ok := m.data[runID]; ok {
		delete(run, nodeID)
	}
	return nil
}

// DeleteRun implements Store.
func (m *MemoryStore) DeleteRun(runID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrStoreClosed
	}

	delete(m.data, runID)
	delete(m.maxSeq, runID)
	return nil
}

// Close implements Store.
func (m *MemoryStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	m.data = nil
	m.maxSeq = nil
	return nil
}

// Len returns the total number of checkpoints across all runs.
// Useful for testing.
func (m *MemoryStore) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, run := range m.data {
		count += len(run)
	}
	return count
}
