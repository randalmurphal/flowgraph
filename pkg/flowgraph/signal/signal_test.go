package signal_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/signal"
)

func TestNewSignal(t *testing.T) {
	sig := signal.NewSignal("test-signal", "run-123", map[string]any{"key": "value"})

	assert.NotEmpty(t, sig.ID)
	assert.Equal(t, "test-signal", sig.Name)
	assert.Equal(t, "run-123", sig.TargetID)
	assert.Equal(t, "value", sig.Payload["key"])
	assert.Equal(t, signal.StatusPending, sig.Status)
	assert.NotZero(t, sig.SentAt)
}

func TestSignal_WithSender(t *testing.T) {
	sig := signal.NewSignal("test", "run-1", nil).WithSender("user-42")
	assert.Equal(t, "user-42", sig.SenderID)
}

func TestSignal_Clone(t *testing.T) {
	sig := signal.NewSignal("test", "run-1", map[string]any{"key": "value"})
	sig.SenderID = "user-1"

	clone := sig.Clone()

	// Verify copy
	assert.Equal(t, sig.ID, clone.ID)
	assert.Equal(t, sig.Name, clone.Name)
	assert.Equal(t, sig.Payload["key"], clone.Payload["key"])

	// Verify independence
	clone.Payload["key"] = "modified"
	assert.Equal(t, "value", sig.Payload["key"])
}

func TestRegistry_Register(t *testing.T) {
	registry := signal.NewRegistry()

	handler := func(_ context.Context, _ string, _ *signal.Signal) error {
		return nil
	}

	err := registry.Register("test-signal", handler)
	require.NoError(t, err)

	// Duplicate registration should fail
	err = registry.Register("test-signal", handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Register_Validation(t *testing.T) {
	registry := signal.NewRegistry()

	t.Run("empty name", func(t *testing.T) {
		err := registry.Register("", func(_ context.Context, _ string, _ *signal.Signal) error { return nil })
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("nil handler", func(t *testing.T) {
		err := registry.Register("test", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler is required")
	})
}

func TestRegistry_MustRegister(t *testing.T) {
	registry := signal.NewRegistry()

	// Should not panic
	registry.MustRegister("test", func(_ context.Context, _ string, _ *signal.Signal) error { return nil })

	// Should panic on duplicate
	assert.Panics(t, func() {
		registry.MustRegister("test", func(_ context.Context, _ string, _ *signal.Signal) error { return nil })
	})
}

func TestRegistry_Get(t *testing.T) {
	registry := signal.NewRegistry()

	called := false
	handler := func(_ context.Context, _ string, _ *signal.Signal) error {
		called = true
		return nil
	}

	_ = registry.Register("test-signal", handler)

	gotHandler, exists := registry.Get("test-signal")
	assert.True(t, exists)
	require.NotNil(t, gotHandler)

	// Verify it's the right handler
	_ = gotHandler(context.Background(), "run-1", &signal.Signal{})
	assert.True(t, called)

	// Non-existent
	_, exists = registry.Get("nonexistent")
	assert.False(t, exists)
}

func TestRegistry_List(t *testing.T) {
	registry := signal.NewRegistry()

	_ = registry.Register("signal-a", func(_ context.Context, _ string, _ *signal.Signal) error { return nil })
	_ = registry.Register("signal-b", func(_ context.Context, _ string, _ *signal.Signal) error { return nil })

	names := registry.List()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "signal-a")
	assert.Contains(t, names, "signal-b")
}

func TestRegistry_Unregister(t *testing.T) {
	registry := signal.NewRegistry()

	_ = registry.Register("test-signal", func(_ context.Context, _ string, _ *signal.Signal) error { return nil })

	registry.Unregister("test-signal")

	_, exists := registry.Get("test-signal")
	assert.False(t, exists)
}

func TestMemoryStore_Enqueue(t *testing.T) {
	store := signal.NewMemoryStore()
	ctx := context.Background()

	sig := signal.NewSignal("test-signal", "run-123", map[string]any{"key": "value"})

	err := store.Enqueue(ctx, sig)
	require.NoError(t, err)

	// Verify stored
	got, err := store.Get(ctx, sig.ID)
	require.NoError(t, err)
	assert.Equal(t, sig.ID, got.ID)
	assert.Equal(t, sig.Name, got.Name)
}

func TestMemoryStore_Dequeue(t *testing.T) {
	store := signal.NewMemoryStore()
	ctx := context.Background()

	// Enqueue signals
	for i := 0; i < 3; i++ {
		_ = store.Enqueue(ctx, signal.NewSignal("test", "run-123", nil))
	}

	// Dequeue should return pending signals
	signals, err := store.Dequeue(ctx, "run-123")
	require.NoError(t, err)
	assert.Len(t, signals, 3)

	// Empty target should return empty
	signals, err = store.Dequeue(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, signals)
}

func TestMemoryStore_MarkProcessed(t *testing.T) {
	store := signal.NewMemoryStore()
	ctx := context.Background()

	sig := signal.NewSignal("test", "run-123", nil)
	_ = store.Enqueue(ctx, sig)

	err := store.MarkProcessed(ctx, sig.ID)
	require.NoError(t, err)

	// Should no longer be dequeued
	signals, _ := store.Dequeue(ctx, "run-123")
	assert.Empty(t, signals)

	// Get should show processed status
	got, _ := store.Get(ctx, sig.ID)
	assert.Equal(t, signal.StatusProcessed, got.Status)
	assert.NotNil(t, got.ProcessedAt)
}

func TestMemoryStore_MarkFailed(t *testing.T) {
	store := signal.NewMemoryStore()
	ctx := context.Background()

	sig := signal.NewSignal("test", "run-123", nil)
	_ = store.Enqueue(ctx, sig)

	err := store.MarkFailed(ctx, sig.ID, errors.New("handler failed"))
	require.NoError(t, err)

	got, _ := store.Get(ctx, sig.ID)
	assert.Equal(t, signal.StatusFailed, got.Status)
	assert.Equal(t, "handler failed", got.Error)
}

func TestMemoryStore_Get_NotFound(t *testing.T) {
	store := signal.NewMemoryStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	assert.ErrorIs(t, err, signal.ErrSignalNotFound)
}

func TestMemoryStore_ListByTarget(t *testing.T) {
	store := signal.NewMemoryStore()
	ctx := context.Background()

	// Add signals for different targets
	_ = store.Enqueue(ctx, signal.NewSignal("s1", "run-1", nil))
	_ = store.Enqueue(ctx, signal.NewSignal("s2", "run-1", nil))
	_ = store.Enqueue(ctx, signal.NewSignal("s3", "run-2", nil))

	signals, err := store.ListByTarget(ctx, "run-1")
	require.NoError(t, err)
	assert.Len(t, signals, 2)

	signals, err = store.ListByTarget(ctx, "run-2")
	require.NoError(t, err)
	assert.Len(t, signals, 1)
}

func TestMemoryStore_Delete(t *testing.T) {
	store := signal.NewMemoryStore()
	ctx := context.Background()

	sig := signal.NewSignal("test", "run-123", nil)
	_ = store.Enqueue(ctx, sig)

	err := store.Delete(ctx, sig.ID)
	require.NoError(t, err)

	_, err = store.Get(ctx, sig.ID)
	assert.ErrorIs(t, err, signal.ErrSignalNotFound)
}

func TestDispatcher_Send(t *testing.T) {
	registry := signal.NewRegistry()
	store := signal.NewMemoryStore()
	dispatcher := signal.NewDispatcher(registry, store)

	ctx := context.Background()

	sig := signal.NewSignal("test-signal", "run-123", map[string]any{"action": "do-something"})

	err := dispatcher.Send(ctx, sig)
	require.NoError(t, err)

	// Should be in store
	signals, _ := store.Dequeue(ctx, "run-123")
	assert.Len(t, signals, 1)
}

func TestDispatcher_Send_Validation(t *testing.T) {
	registry := signal.NewRegistry()
	store := signal.NewMemoryStore()
	dispatcher := signal.NewDispatcher(registry, store)

	ctx := context.Background()

	t.Run("missing target ID", func(t *testing.T) {
		err := dispatcher.Send(ctx, &signal.Signal{Name: "test"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target ID is required")
	})

	t.Run("missing name", func(t *testing.T) {
		err := dispatcher.Send(ctx, &signal.Signal{TargetID: "run-1"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signal name is required")
	})
}

func TestDispatcher_Process(t *testing.T) {
	registry := signal.NewRegistry()
	store := signal.NewMemoryStore()
	dispatcher := signal.NewDispatcher(registry, store)

	ctx := context.Background()

	// Register handler
	var processedSignals []*signal.Signal
	_ = registry.Register("test-signal", func(_ context.Context, _ string, s *signal.Signal) error {
		processedSignals = append(processedSignals, s)
		return nil
	})

	// Enqueue signals
	for i := 0; i < 3; i++ {
		_ = store.Enqueue(ctx, signal.NewSignal("test-signal", "run-123", nil))
	}

	// Process
	err := dispatcher.Process(ctx, "run-123")
	require.NoError(t, err)

	assert.Len(t, processedSignals, 3)

	// All should be marked processed
	signals, _ := store.Dequeue(ctx, "run-123")
	assert.Empty(t, signals)
}

func TestDispatcher_Process_NoHandler(t *testing.T) {
	registry := signal.NewRegistry()
	store := signal.NewMemoryStore()
	dispatcher := signal.NewDispatcher(registry, store)

	ctx := context.Background()

	// Enqueue signal with no handler
	sig := signal.NewSignal("unknown-signal", "run-123", nil)
	_ = store.Enqueue(ctx, sig)

	// Process - should mark as failed
	err := dispatcher.Process(ctx, "run-123")
	require.NoError(t, err) // Process itself doesn't error

	got, _ := store.Get(ctx, sig.ID)
	assert.Equal(t, signal.StatusFailed, got.Status)
	assert.Contains(t, got.Error, "no handler")
}

func TestDispatcher_Process_HandlerError(t *testing.T) {
	registry := signal.NewRegistry()
	store := signal.NewMemoryStore()
	dispatcher := signal.NewDispatcher(registry, store)

	ctx := context.Background()

	// Register failing handler
	_ = registry.Register("failing-signal", func(_ context.Context, _ string, _ *signal.Signal) error {
		return errors.New("handler exploded")
	})

	sig := signal.NewSignal("failing-signal", "run-123", nil)
	_ = store.Enqueue(ctx, sig)

	// Process - should mark as failed
	err := dispatcher.Process(ctx, "run-123")
	require.NoError(t, err)

	got, _ := store.Get(ctx, sig.ID)
	assert.Equal(t, signal.StatusFailed, got.Status)
	assert.Equal(t, "handler exploded", got.Error)
}

func TestDispatcher_ProcessOne(t *testing.T) {
	registry := signal.NewRegistry()
	store := signal.NewMemoryStore()
	dispatcher := signal.NewDispatcher(registry, store)

	ctx := context.Background()

	processed := false
	_ = registry.Register("test-signal", func(_ context.Context, _ string, _ *signal.Signal) error {
		processed = true
		return nil
	})

	sig := signal.NewSignal("test-signal", "run-123", nil)
	_ = store.Enqueue(ctx, sig)

	err := dispatcher.ProcessOne(ctx, sig.ID)
	require.NoError(t, err)
	assert.True(t, processed)
}
