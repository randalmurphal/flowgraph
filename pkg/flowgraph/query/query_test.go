package query_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/query"
)

func TestRegistry_Register(t *testing.T) {
	registry := query.NewRegistry()

	handler := func(_ context.Context, _ string, _ any) (any, error) {
		return "result", nil
	}

	err := registry.Register("test-query", handler)
	require.NoError(t, err)

	// Duplicate registration should fail
	err = registry.Register("test-query", handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Register_Validation(t *testing.T) {
	registry := query.NewRegistry()

	t.Run("empty name", func(t *testing.T) {
		err := registry.Register("", func(_ context.Context, _ string, _ any) (any, error) { return "ok", nil })
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
	registry := query.NewRegistry()

	// Should not panic
	registry.MustRegister("test", func(_ context.Context, _ string, _ any) (any, error) { return "ok", nil })

	// Should panic on duplicate
	assert.Panics(t, func() {
		registry.MustRegister("test", func(_ context.Context, _ string, _ any) (any, error) { return "ok", nil })
	})
}

func TestRegistry_Get(t *testing.T) {
	registry := query.NewRegistry()

	expected := "test-result"
	handler := func(_ context.Context, _ string, _ any) (any, error) {
		return expected, nil
	}

	_ = registry.Register("test-query", handler)

	gotHandler, exists := registry.Get("test-query")
	assert.True(t, exists)
	require.NotNil(t, gotHandler)

	// Verify it's the right handler
	result, err := gotHandler(context.Background(), "run-1", nil)
	require.NoError(t, err)
	assert.Equal(t, expected, result)

	// Non-existent
	_, exists = registry.Get("nonexistent")
	assert.False(t, exists)
}

func TestRegistry_List(t *testing.T) {
	registry := query.NewRegistry()

	_ = registry.Register("query-a", func(_ context.Context, _ string, _ any) (any, error) { return "ok", nil })
	_ = registry.Register("query-b", func(_ context.Context, _ string, _ any) (any, error) { return "ok", nil })

	names := registry.List()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "query-a")
	assert.Contains(t, names, "query-b")
}

func TestRegistry_Unregister(t *testing.T) {
	registry := query.NewRegistry()

	_ = registry.Register("test-query", func(_ context.Context, _ string, _ any) (any, error) { return "ok", nil })

	registry.Unregister("test-query")

	_, exists := registry.Get("test-query")
	assert.False(t, exists)
}

func TestExecutor_Execute(t *testing.T) {
	registry := query.NewRegistry()

	_ = registry.Register("test-query", func(_ context.Context, targetID string, args any) (any, error) {
		return map[string]any{
			"target_id": targetID,
			"args":      args,
		}, nil
	})

	stateLoader := func(_ context.Context, _ string) (*query.State, error) {
		return &query.State{}, nil
	}

	executor := query.NewExecutor(registry, stateLoader)

	ctx := context.Background()
	result, err := executor.Execute(ctx, "run-123", "test-query", "test-args")
	require.NoError(t, err)

	resultMap := result.(map[string]any)
	assert.Equal(t, "run-123", resultMap["target_id"])
	assert.Equal(t, "test-args", resultMap["args"])
}

func TestExecutor_Execute_Validation(t *testing.T) {
	registry := query.NewRegistry()
	executor := query.NewExecutor(registry, nil)

	ctx := context.Background()

	t.Run("missing target ID", func(t *testing.T) {
		_, err := executor.Execute(ctx, "", "test", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target ID is required")
	})

	t.Run("missing query name", func(t *testing.T) {
		_, err := executor.Execute(ctx, "run-1", "", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "query name is required")
	})

	t.Run("unknown query", func(t *testing.T) {
		_, err := executor.Execute(ctx, "run-1", "unknown", nil)
		assert.ErrorIs(t, err, query.ErrQueryNotFound)
	})
}

func TestRegisterBuiltins(t *testing.T) {
	registry := query.NewRegistry()

	state := &query.State{
		TargetID:    "run-123",
		Status:      "running",
		CurrentNode: "node-1",
		Progress:    0.5,
		Variables: map[string]any{
			"input":  "value1",
			"output": "value2",
		},
		PendingTask: &query.PendingTask{
			TaskID:      "task-1",
			NodeID:      "node-2",
			Title:       "Review Changes",
			Description: "Please review the proposed changes",
			Assignee:    "user-1",
		},
	}

	stateLoader := func(_ context.Context, targetID string) (*query.State, error) {
		if targetID == "run-123" {
			return state, nil
		}
		return nil, fmt.Errorf("target %q not found", targetID)
	}

	err := query.RegisterBuiltins(registry, stateLoader)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("status", func(t *testing.T) {
		handler, exists := registry.Get(query.QueryStatus)
		require.True(t, exists)

		result, err := handler(ctx, "run-123", nil)
		require.NoError(t, err)
		assert.Equal(t, "running", result)
	})

	t.Run("progress", func(t *testing.T) {
		handler, exists := registry.Get(query.QueryProgress)
		require.True(t, exists)

		result, err := handler(ctx, "run-123", nil)
		require.NoError(t, err)
		assert.Equal(t, 0.5, result)
	})

	t.Run("current_node", func(t *testing.T) {
		handler, exists := registry.Get(query.QueryCurrentNode)
		require.True(t, exists)

		result, err := handler(ctx, "run-123", nil)
		require.NoError(t, err)
		assert.Equal(t, "node-1", result)
	})

	t.Run("variables - all", func(t *testing.T) {
		handler, exists := registry.Get(query.QueryVariables)
		require.True(t, exists)

		result, err := handler(ctx, "run-123", nil)
		require.NoError(t, err)
		vars := result.(map[string]any)
		assert.Equal(t, "value1", vars["input"])
		assert.Equal(t, "value2", vars["output"])
	})

	t.Run("variables - specific", func(t *testing.T) {
		handler, exists := registry.Get(query.QueryVariables)
		require.True(t, exists)

		result, err := handler(ctx, "run-123", "input")
		require.NoError(t, err)
		assert.Equal(t, "value1", result)
	})

	t.Run("variables - not found", func(t *testing.T) {
		handler, exists := registry.Get(query.QueryVariables)
		require.True(t, exists)

		_, err := handler(ctx, "run-123", "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("pending_task", func(t *testing.T) {
		handler, exists := registry.Get(query.QueryPendingTask)
		require.True(t, exists)

		result, err := handler(ctx, "run-123", nil)
		require.NoError(t, err)
		task := result.(*query.PendingTask)
		assert.Equal(t, "task-1", task.TaskID)
		assert.Equal(t, "Review Changes", task.Title)
	})

	t.Run("state", func(t *testing.T) {
		handler, exists := registry.Get(query.QueryState)
		require.True(t, exists)

		result, err := handler(ctx, "run-123", nil)
		require.NoError(t, err)
		resultState := result.(*query.State)
		assert.Equal(t, "run-123", resultState.TargetID)
		assert.Equal(t, "running", resultState.Status)
	})

	t.Run("target not found", func(t *testing.T) {
		handler, exists := registry.Get(query.QueryStatus)
		require.True(t, exists)

		_, err := handler(ctx, "nonexistent", nil)
		assert.Error(t, err)
	})
}

func TestRegisterBuiltins_ErrorPropagation(t *testing.T) {
	registry := query.NewRegistry()

	expectedErr := errors.New("database error")
	stateLoader := func(_ context.Context, _ string) (*query.State, error) {
		return nil, expectedErr
	}

	err := query.RegisterBuiltins(registry, stateLoader)
	require.NoError(t, err)

	ctx := context.Background()
	handler, _ := registry.Get(query.QueryStatus)
	_, err = handler(ctx, "run-123", nil)
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestExecutor_ExecuteMultiple(t *testing.T) {
	registry := query.NewRegistry()

	state := &query.State{
		TargetID:    "run-123",
		Status:      "running",
		Progress:    0.75,
		CurrentNode: "node-5",
	}

	stateLoader := func(_ context.Context, targetID string) (*query.State, error) {
		if targetID == "run-123" {
			return state, nil
		}
		return nil, fmt.Errorf("not found")
	}

	_ = query.RegisterBuiltins(registry, stateLoader)
	executor := query.NewExecutor(registry, stateLoader)

	ctx := context.Background()
	queries := map[string]any{
		query.QueryStatus:      nil,
		query.QueryProgress:    nil,
		query.QueryCurrentNode: nil,
		"unknown_query":        nil,
	}

	results := executor.ExecuteMultiple(ctx, "run-123", queries)

	assert.Len(t, results, 4)

	// Find results by query name
	resultMap := make(map[string]query.Result)
	for _, r := range results {
		resultMap[r.QueryName] = r
	}

	assert.Equal(t, "running", resultMap[query.QueryStatus].Value)
	assert.Equal(t, 0.75, resultMap[query.QueryProgress].Value)
	assert.Equal(t, "node-5", resultMap[query.QueryCurrentNode].Value)
	assert.Contains(t, resultMap["unknown_query"].Error, "not found")
}

func TestQueryConstants(t *testing.T) {
	// Verify constants exist and have expected values
	assert.Equal(t, "status", query.QueryStatus)
	assert.Equal(t, "progress", query.QueryProgress)
	assert.Equal(t, "current_node", query.QueryCurrentNode)
	assert.Equal(t, "variables", query.QueryVariables)
	assert.Equal(t, "pending_task", query.QueryPendingTask)
	assert.Equal(t, "state", query.QueryState)
}
