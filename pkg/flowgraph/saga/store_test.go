package saga

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore_Create(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	exec := &Execution{
		ID:        "test-1",
		SagaName:  "test-saga",
		Status:    StatusRunning,
		StartedAt: time.Now(),
	}

	// Create should succeed
	err := store.Create(ctx, exec)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Duplicate create should fail
	err = store.Create(ctx, exec)
	if err == nil {
		t.Fatal("Expected error for duplicate create")
	}

	// Create without ID should fail
	err = store.Create(ctx, &Execution{})
	if err == nil {
		t.Fatal("Expected error for missing ID")
	}
}

func TestMemoryStore_Update(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	exec := &Execution{
		ID:        "test-1",
		SagaName:  "test-saga",
		Status:    StatusRunning,
		StartedAt: time.Now(),
	}

	// Update non-existent should fail
	err := store.Update(ctx, exec)
	if err == nil {
		t.Fatal("Expected error for update non-existent")
	}

	// Create then update should succeed
	_ = store.Create(ctx, exec)

	exec.Status = StatusCompleted
	err = store.Update(ctx, exec)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	retrieved, _ := store.Get(ctx, "test-1")
	if retrieved.Status != StatusCompleted {
		t.Errorf("Expected status %v, got %v", StatusCompleted, retrieved.Status)
	}
}

func TestMemoryStore_Get(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Get non-existent should return error
	_, err := store.Get(ctx, "non-existent")
	if err != ErrExecutionNotFound {
		t.Fatalf("Expected ErrExecutionNotFound, got %v", err)
	}

	// Create and get
	exec := &Execution{
		ID:       "test-1",
		SagaName: "test-saga",
		Status:   StatusRunning,
	}
	_ = store.Create(ctx, exec)

	retrieved, err := store.Get(ctx, "test-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.ID != "test-1" {
		t.Errorf("Expected ID test-1, got %s", retrieved.ID)
	}

	// Verify it's a clone (not the same pointer)
	retrieved.Status = StatusFailed
	original, _ := store.Get(ctx, "test-1")
	if original.Status == StatusFailed {
		t.Error("Expected clone, not original reference")
	}
}

func TestMemoryStore_List(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create multiple executions
	for i := 1; i <= 5; i++ {
		status := StatusRunning
		if i > 3 {
			status = StatusCompleted
		}
		exec := &Execution{
			ID:       string(rune('0' + i)),
			SagaName: "test-saga",
			Status:   status,
		}
		_ = store.Create(ctx, exec)
	}

	// List all
	all, err := store.List(ctx, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("Expected 5 executions, got %d", len(all))
	}

	// List with status filter
	running, err := store.List(ctx, &ListFilter{Status: StatusRunning})
	if err != nil {
		t.Fatalf("List with filter failed: %v", err)
	}
	if len(running) != 3 {
		t.Errorf("Expected 3 running, got %d", len(running))
	}

	// List with saga name filter
	byName, err := store.List(ctx, &ListFilter{SagaName: "test-saga"})
	if err != nil {
		t.Fatalf("List with name filter failed: %v", err)
	}
	if len(byName) != 5 {
		t.Errorf("Expected 5 with name, got %d", len(byName))
	}

	// List with limit
	limited, err := store.List(ctx, &ListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List with limit failed: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("Expected 2 limited, got %d", len(limited))
	}

	// List with offset beyond size
	offsetBeyond, err := store.List(ctx, &ListFilter{Offset: 10})
	if err != nil {
		t.Fatalf("List with offset failed: %v", err)
	}
	if len(offsetBeyond) != 0 {
		t.Errorf("Expected 0 with offset beyond, got %d", len(offsetBeyond))
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Delete non-existent should fail
	err := store.Delete(ctx, "non-existent")
	if err != ErrExecutionNotFound {
		t.Fatalf("Expected ErrExecutionNotFound, got %v", err)
	}

	// Create and delete
	exec := &Execution{
		ID:       "test-1",
		SagaName: "test-saga",
	}
	_ = store.Create(ctx, exec)

	err = store.Delete(ctx, "test-1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = store.Get(ctx, "test-1")
	if err != ErrExecutionNotFound {
		t.Fatal("Expected execution to be deleted")
	}
}

func TestOrchestrator_WithStore(t *testing.T) {
	store := NewMemoryStore()
	orch := NewOrchestrator(WithStore(store))

	saga := &Definition{
		Name: "test-saga",
		Steps: []Step{
			{
				Name: "step1",
				Handler: func(ctx context.Context, input any) (any, error) {
					return map[string]any{"result": "done"}, nil
				},
			},
		},
	}
	_ = orch.Register(saga)

	ctx := context.Background()
	exec, err := orch.Start(ctx, "test-saga", nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	// Verify execution was persisted to store
	persisted, err := store.Get(ctx, exec.ID)
	if err != nil {
		t.Fatalf("Get from store failed: %v", err)
	}

	if persisted.Status != StatusCompleted {
		t.Errorf("Expected completed status in store, got %v", persisted.Status)
	}

	// Verify orchestrator Get works with store
	fromOrch := orch.Get(exec.ID)
	if fromOrch == nil {
		t.Fatal("Expected to get execution from orchestrator")
	}
	if fromOrch.Status != StatusCompleted {
		t.Errorf("Expected completed status from orchestrator, got %v", fromOrch.Status)
	}
}

func TestOrchestrator_WithStore_Compensation(t *testing.T) {
	store := NewMemoryStore()
	orch := NewOrchestrator(WithStore(store))

	compensated := false
	saga := &Definition{
		Name: "failing-saga",
		Steps: []Step{
			{
				Name: "step1",
				Handler: func(ctx context.Context, input any) (any, error) {
					return "step1-done", nil
				},
				Compensation: func(ctx context.Context, output any) (any, error) {
					compensated = true
					return nil, nil
				},
			},
			{
				Name: "step2-fails",
				Handler: func(ctx context.Context, input any) (any, error) {
					return nil, context.DeadlineExceeded
				},
			},
		},
	}
	_ = orch.Register(saga)

	ctx := context.Background()
	exec, _ := orch.Start(ctx, "failing-saga", nil)

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	// Verify compensation ran
	if !compensated {
		t.Error("Expected compensation to run")
	}

	// Verify final state in store
	persisted, _ := store.Get(ctx, exec.ID)
	if persisted.Status != StatusCompensated {
		t.Errorf("Expected compensated status in store, got %v", persisted.Status)
	}
}
