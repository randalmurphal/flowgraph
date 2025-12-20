package checkpoint_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteStore_Persistence(t *testing.T) {
	// Create temp file for database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// First store instance
	store1, err := checkpoint.NewSQLiteStore(dbPath)
	require.NoError(t, err)

	require.NoError(t, store1.Save("run-1", "node-a", []byte("persistent")))
	require.NoError(t, store1.Close())

	// Second store instance (reopening the database)
	store2, err := checkpoint.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store2.Close()

	// Data should persist
	data, err := store2.Load("run-1", "node-a")
	require.NoError(t, err)
	assert.Equal(t, []byte("persistent"), data)
}

func TestSQLiteStore_InvalidPath(t *testing.T) {
	// Try to create in non-existent directory
	_, err := checkpoint.NewSQLiteStore("/nonexistent/path/db.sqlite")
	assert.Error(t, err)
}

func TestSQLiteStore_CloseIdempotent(t *testing.T) {
	store, err := checkpoint.NewSQLiteStore(":memory:")
	require.NoError(t, err)

	// Close multiple times should be safe
	assert.NoError(t, store.Close())
	assert.NoError(t, store.Close())
}

func TestSQLiteStore_Concurrent(t *testing.T) {
	store, err := checkpoint.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	const numGoroutines = 50
	const numOps = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			runID := "run-" + string(rune('a'+id%26))
			for j := 0; j < numOps; j++ {
				nodeID := "node-" + string(rune('0'+j%10))
				data := []byte("data")

				switch j % 4 {
				case 0, 1:
					_ = store.Save(runID, nodeID, data)
				case 2:
					_, _ = store.Load(runID, nodeID)
				case 3:
					_, _ = store.List(runID)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestSQLiteStore_LargeData(t *testing.T) {
	store, err := checkpoint.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	// 1MB of data
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	require.NoError(t, store.Save("run-1", "large", largeData))

	loaded, err := store.Load("run-1", "large")
	require.NoError(t, err)
	assert.Equal(t, largeData, loaded)

	// Verify size in listing
	infos, err := store.List("run-1")
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.Equal(t, int64(1024*1024), infos[0].Size)
}

func TestSQLiteStore_FileSizeGrowth(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "growth.db")

	store, err := checkpoint.NewSQLiteStore(dbPath)
	require.NoError(t, err)

	// Save some data
	for i := 0; i < 10; i++ {
		data := make([]byte, 10000) // 10KB each
		require.NoError(t, store.Save("run-1", "node-"+string(rune('a'+i)), data))
	}

	require.NoError(t, store.Close())

	// Check file exists and has reasonable size
	info, err := os.Stat(dbPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(50000)) // Should be at least 50KB
}

func TestSQLiteStore_SequenceOnUpdate(t *testing.T) {
	store, err := checkpoint.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	// Save checkpoints
	require.NoError(t, store.Save("run-1", "node-a", []byte("first")))
	require.NoError(t, store.Save("run-1", "node-b", []byte("second")))

	// Update existing checkpoint
	require.NoError(t, store.Save("run-1", "node-a", []byte("updated")))

	// List should show updated sequence for node-a
	infos, err := store.List("run-1")
	require.NoError(t, err)
	require.Len(t, infos, 2)

	// node-b should come first (sequence 2), node-a should be last (sequence 3 after update)
	assert.Equal(t, "node-b", infos[0].NodeID)
	assert.Equal(t, 2, infos[0].Sequence)
	assert.Equal(t, "node-a", infos[1].NodeID)
	assert.Equal(t, 3, infos[1].Sequence)
}
