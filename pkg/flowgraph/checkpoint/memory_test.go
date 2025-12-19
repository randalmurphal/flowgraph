package checkpoint_test

import (
	"sync"
	"testing"

	"github.com/rmurphy/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore_Len(t *testing.T) {
	store := checkpoint.NewMemoryStore()
	defer store.Close()

	assert.Equal(t, 0, store.Len())

	require.NoError(t, store.Save("run-1", "node-a", []byte("a")))
	assert.Equal(t, 1, store.Len())

	require.NoError(t, store.Save("run-1", "node-b", []byte("b")))
	assert.Equal(t, 2, store.Len())

	require.NoError(t, store.Save("run-2", "node-a", []byte("x")))
	assert.Equal(t, 3, store.Len())

	require.NoError(t, store.Delete("run-1", "node-a"))
	assert.Equal(t, 2, store.Len())

	require.NoError(t, store.DeleteRun("run-1"))
	assert.Equal(t, 1, store.Len())
}

func TestMemoryStore_Concurrent(t *testing.T) {
	store := checkpoint.NewMemoryStore()
	defer store.Close()

	const numGoroutines = 100
	const numOps = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			runID := "run-" + string(rune('a'+id%26))
			for j := 0; j < numOps; j++ {
				nodeID := "node-" + string(rune('0'+j%10))
				data := []byte("data")

				// Mix of operations
				switch j % 5 {
				case 0, 1:
					_ = store.Save(runID, nodeID, data)
				case 2:
					_, _ = store.Load(runID, nodeID)
				case 3:
					_, _ = store.List(runID)
				case 4:
					_ = store.Delete(runID, nodeID)
				}
			}
		}(i)
	}

	wg.Wait()

	// Should not panic or deadlock
	// Final state doesn't matter, just verifying concurrent safety
}

func TestMemoryStore_InfoMetadata(t *testing.T) {
	store := checkpoint.NewMemoryStore()
	defer store.Close()

	require.NoError(t, store.Save("run-1", "node-a", []byte("short")))

	infos, err := store.List("run-1")
	require.NoError(t, err)
	require.Len(t, infos, 1)

	info := infos[0]
	assert.Equal(t, "run-1", info.RunID)
	assert.Equal(t, "node-a", info.NodeID)
	assert.Equal(t, int64(5), info.Size) // len("short")
	assert.False(t, info.Timestamp.IsZero())
}
