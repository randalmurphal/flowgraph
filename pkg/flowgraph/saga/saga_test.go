package saga_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/saga"
)

func TestDefinition_Validate(t *testing.T) {
	t.Run("valid saga", func(t *testing.T) {
		def := &saga.Definition{
			Name: "test-saga",
			Steps: []saga.Step{
				{Name: "step1", Handler: func(_ context.Context, _ any) (any, error) { return "ok", nil }},
			},
		}
		err := def.Validate()
		require.NoError(t, err)
	})

	t.Run("empty name", func(t *testing.T) {
		def := &saga.Definition{
			Steps: []saga.Step{{Name: "step1", Handler: func(_ context.Context, _ any) (any, error) { return "ok", nil }}},
		}
		err := def.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("no steps", func(t *testing.T) {
		def := &saga.Definition{Name: "test"}
		err := def.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one step")
	})

	t.Run("step without name", func(t *testing.T) {
		def := &saga.Definition{
			Name: "test",
			Steps: []saga.Step{
				{Handler: func(_ context.Context, _ any) (any, error) { return "ok", nil }},
			},
		}
		err := def.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("step without handler", func(t *testing.T) {
		def := &saga.Definition{
			Name: "test",
			Steps: []saga.Step{
				{Name: "step1"},
			},
		}
		err := def.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler is required")
	})
}

func TestOrchestrator_Register(t *testing.T) {
	orch := saga.NewOrchestrator()

	def := &saga.Definition{
		Name: "test-saga",
		Steps: []saga.Step{
			{Name: "step1", Handler: func(_ context.Context, _ any) (any, error) { return "ok", nil }},
		},
	}

	err := orch.Register(def)
	require.NoError(t, err)

	// Duplicate registration should fail
	err = orch.Register(def)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestOrchestrator_MustRegister(t *testing.T) {
	orch := saga.NewOrchestrator()

	def := &saga.Definition{
		Name: "test-saga",
		Steps: []saga.Step{
			{Name: "step1", Handler: func(_ context.Context, _ any) (any, error) { return "ok", nil }},
		},
	}

	// Should not panic
	orch.MustRegister(def)

	// Should panic on duplicate
	assert.Panics(t, func() {
		orch.MustRegister(def)
	})
}

func TestOrchestrator_Start_Success(t *testing.T) {
	orch := saga.NewOrchestrator()

	var executedSteps []string
	var mu sync.Mutex

	def := &saga.Definition{
		Name:    "order-saga",
		Timeout: 5 * time.Second,
		Steps: []saga.Step{
			{
				Name: "create-order",
				Handler: func(_ context.Context, input any) (any, error) {
					mu.Lock()
					executedSteps = append(executedSteps, "create-order")
					mu.Unlock()
					return map[string]any{"order_id": "ORD-123", "input": input}, nil
				},
			},
			{
				Name: "reserve-inventory",
				Handler: func(_ context.Context, input any) (any, error) {
					mu.Lock()
					executedSteps = append(executedSteps, "reserve-inventory")
					mu.Unlock()
					data := input.(map[string]any)
					return map[string]any{"order_id": data["order_id"], "reserved": true}, nil
				},
			},
			{
				Name: "charge-payment",
				Handler: func(_ context.Context, input any) (any, error) {
					mu.Lock()
					executedSteps = append(executedSteps, "charge-payment")
					mu.Unlock()
					data := input.(map[string]any)
					return map[string]any{"order_id": data["order_id"], "charged": true}, nil
				},
			},
		},
	}

	err := orch.Register(def)
	require.NoError(t, err)

	ctx := context.Background()
	execution, err := orch.Start(ctx, "order-saga", map[string]any{"user_id": "user-1"})
	require.NoError(t, err)
	require.NotNil(t, execution)

	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	// Check execution status
	exec := orch.Get(execution.ID)
	require.NotNil(t, exec)
	assert.Equal(t, saga.StatusCompleted, exec.Status)
	assert.Len(t, exec.Steps, 3)

	// Verify all steps executed
	mu.Lock()
	assert.Equal(t, []string{"create-order", "reserve-inventory", "charge-payment"}, executedSteps)
	mu.Unlock()

	// Check step outputs
	for _, step := range exec.Steps {
		assert.Equal(t, saga.StatusCompleted, step.Status)
		assert.NotNil(t, step.Output)
	}
}

func TestOrchestrator_Start_FailureWithCompensation(t *testing.T) {
	orch := saga.NewOrchestrator()

	var executedSteps []string
	var compensatedSteps []string
	var mu sync.Mutex

	def := &saga.Definition{
		Name:    "failing-saga",
		Timeout: 5 * time.Second,
		Steps: []saga.Step{
			{
				Name: "step1",
				Handler: func(_ context.Context, _ any) (any, error) {
					mu.Lock()
					executedSteps = append(executedSteps, "step1")
					mu.Unlock()
					return "result1", nil
				},
				Compensation: func(_ context.Context, _ any) (any, error) {
					mu.Lock()
					compensatedSteps = append(compensatedSteps, "step1")
					mu.Unlock()
					return "compensated", nil
				},
			},
			{
				Name: "step2",
				Handler: func(_ context.Context, _ any) (any, error) {
					mu.Lock()
					executedSteps = append(executedSteps, "step2")
					mu.Unlock()
					return "result2", nil
				},
				Compensation: func(_ context.Context, _ any) (any, error) {
					mu.Lock()
					compensatedSteps = append(compensatedSteps, "step2")
					mu.Unlock()
					return "ok", nil
				},
			},
			{
				Name: "step3-fails",
				Handler: func(_ context.Context, _ any) (any, error) {
					mu.Lock()
					executedSteps = append(executedSteps, "step3")
					mu.Unlock()
					return nil, errors.New("step3 failed")
				},
			},
		},
	}

	err := orch.Register(def)
	require.NoError(t, err)

	ctx := context.Background()
	execution, err := orch.Start(ctx, "failing-saga", nil)
	require.NoError(t, err)

	// Wait for compensation to complete
	time.Sleep(200 * time.Millisecond)

	exec := orch.Get(execution.ID)
	require.NotNil(t, exec)
	assert.Equal(t, saga.StatusCompensated, exec.Status)

	// Verify execution order
	mu.Lock()
	assert.Equal(t, []string{"step1", "step2", "step3"}, executedSteps)
	// Compensation runs in reverse order
	assert.Equal(t, []string{"step2", "step1"}, compensatedSteps)
	mu.Unlock()
}

func TestOrchestrator_Start_OptionalStep(t *testing.T) {
	orch := saga.NewOrchestrator()

	var executedSteps []string
	var mu sync.Mutex

	def := &saga.Definition{
		Name: "optional-saga",
		Steps: []saga.Step{
			{
				Name: "step1",
				Handler: func(_ context.Context, _ any) (any, error) {
					mu.Lock()
					executedSteps = append(executedSteps, "step1")
					mu.Unlock()
					return "result1", nil
				},
			},
			{
				Name:     "optional-step",
				Optional: true,
				Handler: func(_ context.Context, _ any) (any, error) {
					mu.Lock()
					executedSteps = append(executedSteps, "optional")
					mu.Unlock()
					return nil, errors.New("optional step failed")
				},
			},
			{
				Name: "step3",
				Handler: func(_ context.Context, _ any) (any, error) {
					mu.Lock()
					executedSteps = append(executedSteps, "step3")
					mu.Unlock()
					return "result3", nil
				},
			},
		},
	}

	err := orch.Register(def)
	require.NoError(t, err)

	ctx := context.Background()
	execution, err := orch.Start(ctx, "optional-saga", nil)
	require.NoError(t, err)

	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	exec := orch.Get(execution.ID)
	require.NotNil(t, exec)
	// Should complete even with optional step failure
	assert.Equal(t, saga.StatusCompleted, exec.Status)

	mu.Lock()
	assert.Equal(t, []string{"step1", "optional", "step3"}, executedSteps)
	mu.Unlock()
}

func TestOrchestrator_Compensate(t *testing.T) {
	orch := saga.NewOrchestrator()

	var compensated bool
	var mu sync.Mutex

	def := &saga.Definition{
		Name: "compensate-saga",
		Steps: []saga.Step{
			{
				Name: "step1",
				Handler: func(_ context.Context, _ any) (any, error) {
					return "result", nil
				},
				Compensation: func(_ context.Context, _ any) (any, error) {
					mu.Lock()
					compensated = true
					mu.Unlock()
					return "ok", nil
				},
			},
		},
	}

	_ = orch.Register(def)

	ctx := context.Background()
	execution, _ := orch.Start(ctx, "compensate-saga", nil)

	// Wait for completion
	time.Sleep(50 * time.Millisecond)

	// Manually trigger compensation
	err := orch.Compensate(ctx, execution.ID, "manual rollback")
	require.NoError(t, err)

	// Wait for compensation
	time.Sleep(50 * time.Millisecond)

	exec := orch.Get(execution.ID)
	require.NotNil(t, exec)
	assert.Equal(t, saga.StatusCompensated, exec.Status)

	mu.Lock()
	assert.True(t, compensated)
	mu.Unlock()
}

func TestOrchestrator_Get_NotFound(t *testing.T) {
	orch := saga.NewOrchestrator()

	exec := orch.Get("nonexistent")
	assert.Nil(t, exec)
}

func TestOrchestrator_List(t *testing.T) {
	orch := saga.NewOrchestrator()

	def := &saga.Definition{
		Name: "list-saga",
		Steps: []saga.Step{
			{Name: "step1", Handler: func(_ context.Context, _ any) (any, error) { return "ok", nil }},
		},
	}
	_ = orch.Register(def)

	ctx := context.Background()
	_, _ = orch.Start(ctx, "list-saga", nil)
	_, _ = orch.Start(ctx, "list-saga", nil)

	// Wait for executions
	time.Sleep(50 * time.Millisecond)

	list := orch.List()
	assert.Len(t, list, 2)
}

func TestOrchestrator_ListByStatus(t *testing.T) {
	orch := saga.NewOrchestrator()

	def := &saga.Definition{
		Name: "status-saga",
		Steps: []saga.Step{
			{Name: "step1", Handler: func(_ context.Context, _ any) (any, error) { return "ok", nil }},
		},
	}
	_ = orch.Register(def)

	ctx := context.Background()
	_, _ = orch.Start(ctx, "status-saga", nil)
	_, _ = orch.Start(ctx, "status-saga", nil)

	// Wait for completion
	time.Sleep(50 * time.Millisecond)

	completed := orch.ListByStatus(saga.StatusCompleted)
	assert.Len(t, completed, 2)

	running := orch.ListByStatus(saga.StatusRunning)
	assert.Empty(t, running)
}

func TestOrchestrator_Remove(t *testing.T) {
	orch := saga.NewOrchestrator()

	def := &saga.Definition{
		Name: "remove-saga",
		Steps: []saga.Step{
			{Name: "step1", Handler: func(_ context.Context, _ any) (any, error) { return "ok", nil }},
		},
	}
	_ = orch.Register(def)

	ctx := context.Background()
	execution, _ := orch.Start(ctx, "remove-saga", nil)

	// Wait for completion
	time.Sleep(50 * time.Millisecond)

	err := orch.Remove(execution.ID)
	require.NoError(t, err)

	// Should be gone
	exec := orch.Get(execution.ID)
	assert.Nil(t, exec)
}

func TestOrchestrator_Remove_NotFound(t *testing.T) {
	orch := saga.NewOrchestrator()

	err := orch.Remove("nonexistent")
	assert.Error(t, err)
}

func TestOrchestrator_GetRegistered(t *testing.T) {
	orch := saga.NewOrchestrator()

	def := &saga.Definition{
		Name: "registered-saga",
		Steps: []saga.Step{
			{Name: "step1", Handler: func(_ context.Context, _ any) (any, error) { return "ok", nil }},
		},
	}
	_ = orch.Register(def)

	registered := orch.GetRegistered("registered-saga")
	require.NotNil(t, registered)
	assert.Equal(t, "registered-saga", registered.Name)

	notFound := orch.GetRegistered("nonexistent")
	assert.Nil(t, notFound)
}

func TestOrchestrator_ListRegistered(t *testing.T) {
	orch := saga.NewOrchestrator()

	_ = orch.Register(&saga.Definition{
		Name:  "saga1",
		Steps: []saga.Step{{Name: "s1", Handler: func(_ context.Context, _ any) (any, error) { return "ok", nil }}},
	})
	_ = orch.Register(&saga.Definition{
		Name:  "saga2",
		Steps: []saga.Step{{Name: "s1", Handler: func(_ context.Context, _ any) (any, error) { return "ok", nil }}},
	})

	names := orch.ListRegistered()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "saga1")
	assert.Contains(t, names, "saga2")
}

func TestOrchestrator_Start_NotFound(t *testing.T) {
	orch := saga.NewOrchestrator()

	ctx := context.Background()
	_, err := orch.Start(ctx, "nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExecution_Clone(t *testing.T) {
	exec := &saga.Execution{
		ID:       "test-id",
		SagaName: "test-saga",
		Status:   saga.StatusCompleted,
		Steps: []saga.StepExecution{
			{StepName: "step1", Status: saga.StatusCompleted},
		},
	}

	clone := exec.Clone()

	assert.Equal(t, exec.ID, clone.ID)
	assert.Equal(t, exec.SagaName, clone.SagaName)
	assert.Equal(t, exec.Status, clone.Status)
	assert.Len(t, clone.Steps, 1)

	// Verify independence
	clone.Steps[0].StepName = "modified"
	assert.Equal(t, "step1", exec.Steps[0].StepName)
}

func TestOrchestrator_OnComplete_Callback(t *testing.T) {
	orch := saga.NewOrchestrator()

	var callbackExec *saga.Execution
	var mu sync.Mutex

	def := &saga.Definition{
		Name: "callback-saga",
		Steps: []saga.Step{
			{Name: "step1", Handler: func(_ context.Context, _ any) (any, error) { return "ok", nil }},
		},
		OnComplete: func(_ context.Context, exec *saga.Execution) {
			mu.Lock()
			callbackExec = exec
			mu.Unlock()
		},
	}
	_ = orch.Register(def)

	ctx := context.Background()
	_, _ = orch.Start(ctx, "callback-saga", nil)

	// Wait for completion
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	require.NotNil(t, callbackExec)
	assert.Equal(t, saga.StatusCompleted, callbackExec.Status)
	mu.Unlock()
}
