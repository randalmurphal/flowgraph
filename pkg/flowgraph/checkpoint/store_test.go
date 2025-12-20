package checkpoint_test

import (
	"testing"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// storeFactory creates a store instance for testing.
type storeFactory func(t *testing.T) checkpoint.Store

// storeContractTest runs contract tests against any Store implementation.
func storeContractTest(t *testing.T, name string, factory storeFactory) {
	t.Run(name+"/Save_and_Load", func(t *testing.T) {
		store := factory(t)
		defer store.Close()

		data := []byte(`{"key": "value"}`)
		err := store.Save("run-1", "node-a", data)
		require.NoError(t, err)

		loaded, err := store.Load("run-1", "node-a")
		require.NoError(t, err)
		assert.Equal(t, data, loaded)
	})

	t.Run(name+"/Load_NotFound", func(t *testing.T) {
		store := factory(t)
		defer store.Close()

		_, err := store.Load("run-nonexistent", "node-nonexistent")
		assert.ErrorIs(t, err, checkpoint.ErrNotFound)
	})

	t.Run(name+"/Save_Overwrite", func(t *testing.T) {
		store := factory(t)
		defer store.Close()

		err := store.Save("run-1", "node-a", []byte("first"))
		require.NoError(t, err)

		err = store.Save("run-1", "node-a", []byte("second"))
		require.NoError(t, err)

		loaded, err := store.Load("run-1", "node-a")
		require.NoError(t, err)
		assert.Equal(t, []byte("second"), loaded)
	})

	t.Run(name+"/List_Empty", func(t *testing.T) {
		store := factory(t)
		defer store.Close()

		infos, err := store.List("run-nonexistent")
		require.NoError(t, err)
		assert.Empty(t, infos)
	})

	t.Run(name+"/List_Ordered", func(t *testing.T) {
		store := factory(t)
		defer store.Close()

		// Save in order
		require.NoError(t, store.Save("run-1", "node-a", []byte("a")))
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
		require.NoError(t, store.Save("run-1", "node-b", []byte("bb")))
		time.Sleep(10 * time.Millisecond)
		require.NoError(t, store.Save("run-1", "node-c", []byte("ccc")))

		infos, err := store.List("run-1")
		require.NoError(t, err)
		require.Len(t, infos, 3)

		// Should be ordered by sequence
		assert.Equal(t, 1, infos[0].Sequence)
		assert.Equal(t, 2, infos[1].Sequence)
		assert.Equal(t, 3, infos[2].Sequence)

		// Check node IDs
		assert.Equal(t, "node-a", infos[0].NodeID)
		assert.Equal(t, "node-b", infos[1].NodeID)
		assert.Equal(t, "node-c", infos[2].NodeID)

		// Check sizes
		assert.Equal(t, int64(1), infos[0].Size)
		assert.Equal(t, int64(2), infos[1].Size)
		assert.Equal(t, int64(3), infos[2].Size)
	})

	t.Run(name+"/Delete", func(t *testing.T) {
		store := factory(t)
		defer store.Close()

		require.NoError(t, store.Save("run-1", "node-a", []byte("data")))
		require.NoError(t, store.Delete("run-1", "node-a"))

		_, err := store.Load("run-1", "node-a")
		assert.ErrorIs(t, err, checkpoint.ErrNotFound)
	})

	t.Run(name+"/Delete_Nonexistent", func(t *testing.T) {
		store := factory(t)
		defer store.Close()

		// Should not error when deleting nonexistent
		err := store.Delete("run-nonexistent", "node-nonexistent")
		assert.NoError(t, err)
	})

	t.Run(name+"/DeleteRun", func(t *testing.T) {
		store := factory(t)
		defer store.Close()

		require.NoError(t, store.Save("run-1", "node-a", []byte("a")))
		require.NoError(t, store.Save("run-1", "node-b", []byte("b")))
		require.NoError(t, store.Save("run-2", "node-a", []byte("other")))

		require.NoError(t, store.DeleteRun("run-1"))

		// run-1 checkpoints should be gone
		infos, err := store.List("run-1")
		require.NoError(t, err)
		assert.Empty(t, infos)

		// run-2 should still exist
		infos, err = store.List("run-2")
		require.NoError(t, err)
		assert.Len(t, infos, 1)
	})

	t.Run(name+"/DeleteRun_Nonexistent", func(t *testing.T) {
		store := factory(t)
		defer store.Close()

		// Should not error when deleting nonexistent run
		err := store.DeleteRun("run-nonexistent")
		assert.NoError(t, err)
	})

	t.Run(name+"/MultipleRuns", func(t *testing.T) {
		store := factory(t)
		defer store.Close()

		require.NoError(t, store.Save("run-1", "node-a", []byte("run1-a")))
		require.NoError(t, store.Save("run-1", "node-b", []byte("run1-b")))
		require.NoError(t, store.Save("run-2", "node-a", []byte("run2-a")))

		// Check run-1
		data, err := store.Load("run-1", "node-a")
		require.NoError(t, err)
		assert.Equal(t, []byte("run1-a"), data)

		// Check run-2
		data, err = store.Load("run-2", "node-a")
		require.NoError(t, err)
		assert.Equal(t, []byte("run2-a"), data)

		// Lists are independent
		infos1, _ := store.List("run-1")
		infos2, _ := store.List("run-2")
		assert.Len(t, infos1, 2)
		assert.Len(t, infos2, 1)
	})

	t.Run(name+"/DataCopy", func(t *testing.T) {
		store := factory(t)
		defer store.Close()

		original := []byte("original data")
		require.NoError(t, store.Save("run-1", "node-a", original))

		// Modify original slice after save
		original[0] = 'X'

		// Loaded data should be unchanged
		loaded, err := store.Load("run-1", "node-a")
		require.NoError(t, err)
		assert.Equal(t, []byte("original data"), loaded)
	})

	t.Run(name+"/Close_ThenError", func(t *testing.T) {
		store := factory(t)
		require.NoError(t, store.Close())

		// Operations after close should error
		err := store.Save("run-1", "node-a", []byte("data"))
		assert.ErrorIs(t, err, checkpoint.ErrStoreClosed)

		_, err = store.Load("run-1", "node-a")
		assert.ErrorIs(t, err, checkpoint.ErrStoreClosed)

		_, err = store.List("run-1")
		assert.ErrorIs(t, err, checkpoint.ErrStoreClosed)
	})
}

// TestMemoryStore runs contract tests against MemoryStore.
func TestMemoryStore(t *testing.T) {
	factory := func(t *testing.T) checkpoint.Store {
		return checkpoint.NewMemoryStore()
	}
	storeContractTest(t, "MemoryStore", factory)
}

// TestSQLiteStore runs contract tests against SQLiteStore.
func TestSQLiteStore(t *testing.T) {
	factory := func(t *testing.T) checkpoint.Store {
		store, err := checkpoint.NewSQLiteStore(":memory:")
		require.NoError(t, err)
		return store
	}
	storeContractTest(t, "SQLiteStore", factory)
}
