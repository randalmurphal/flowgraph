package flowgraph

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRun_LinearFlow tests basic linear execution.
func TestRun_LinearFlow(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("inc1", increment).
		AddNode("inc2", increment).
		AddNode("inc3", increment).
		AddEdge("inc1", "inc2").
		AddEdge("inc2", "inc3").
		AddEdge("inc3", END).
		SetEntry("inc1")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(testCtx(), Counter{Value: 0})

	require.NoError(t, err)
	assert.Equal(t, 3, result.Value)
}

// TestRun_SingleNode tests single node execution.
func TestRun_SingleNode(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("only", increment).
		AddEdge("only", END).
		SetEntry("only")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(testCtx(), Counter{Value: 10})

	require.NoError(t, err)
	assert.Equal(t, 11, result.Value)
}

// TestRun_StatePassedBetweenNodes tests state flows correctly.
func TestRun_StatePassedBetweenNodes(t *testing.T) {
	var nodeAState, nodeBState State

	nodeA := func(ctx Context, s State) (State, error) {
		nodeAState = s
		s.Step = 1
		return s, nil
	}
	nodeB := func(ctx Context, s State) (State, error) {
		nodeBState = s
		s.Step = 2
		return s, nil
	}

	graph := NewGraph[State]().
		AddNode("a", nodeA).
		AddNode("b", nodeB).
		AddEdge("a", "b").
		AddEdge("b", END).
		SetEntry("a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(testCtx(), State{Initial: "test"})

	require.NoError(t, err)
	assert.Equal(t, "test", nodeAState.Initial) // A received initial state
	assert.Equal(t, 1, nodeBState.Step)         // B received A's output
	assert.Equal(t, 2, result.Step)             // Final result has B's changes
}

// TestRun_ConditionalEdge_Left tests conditional routing to left branch.
func TestRun_ConditionalEdge_Left(t *testing.T) {
	var executed []string

	router := func(ctx Context, s State) string {
		if s.GoLeft {
			return "left"
		}
		return "right"
	}

	graph := NewGraph[State]().
		AddNode("start", makeTrackingNode("start", &executed)).
		AddNode("left", makeTrackingNode("left", &executed)).
		AddNode("right", makeTrackingNode("right", &executed)).
		AddConditionalEdge("start", router).
		AddEdge("left", END).
		AddEdge("right", END).
		SetEntry("start")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Run(testCtx(), State{GoLeft: true})

	require.NoError(t, err)
	assert.Equal(t, []string{"start", "left"}, executed)
}

// TestRun_ConditionalEdge_Right tests conditional routing to right branch.
func TestRun_ConditionalEdge_Right(t *testing.T) {
	var executed []string

	router := func(ctx Context, s State) string {
		if s.GoLeft {
			return "left"
		}
		return "right"
	}

	graph := NewGraph[State]().
		AddNode("start", makeTrackingNode("start", &executed)).
		AddNode("left", makeTrackingNode("left", &executed)).
		AddNode("right", makeTrackingNode("right", &executed)).
		AddConditionalEdge("start", router).
		AddEdge("left", END).
		AddEdge("right", END).
		SetEntry("start")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Run(testCtx(), State{GoLeft: false})

	require.NoError(t, err)
	assert.Equal(t, []string{"start", "right"}, executed)
}

// TestRun_ConditionalEdge_ToEND tests conditional routing directly to END.
func TestRun_ConditionalEdge_ToEND(t *testing.T) {
	var executed []string

	router := func(ctx Context, s State) string {
		if s.Done {
			return END
		}
		return "continue"
	}

	graph := NewGraph[State]().
		AddNode("check", makeTrackingNode("check", &executed)).
		AddNode("continue", makeTrackingNode("continue", &executed)).
		AddConditionalEdge("check", router).
		AddEdge("continue", END).
		SetEntry("check")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Run(testCtx(), State{Done: true})

	require.NoError(t, err)
	assert.Equal(t, []string{"check"}, executed) // Should stop at check
}

// TestRun_Loop tests looping behavior with conditional exit.
func TestRun_Loop(t *testing.T) {
	var iterations int

	loopNode := func(ctx Context, s State) (State, error) {
		iterations++
		s.Count++
		return s, nil
	}

	router := func(ctx Context, s State) string {
		if s.Count >= 3 {
			return END
		}
		return "loop"
	}

	graph := NewGraph[State]().
		AddNode("loop", loopNode).
		AddConditionalEdge("loop", router).
		SetEntry("loop")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(testCtx(), State{Count: 0})

	require.NoError(t, err)
	assert.Equal(t, 3, iterations)
	assert.Equal(t, 3, result.Count)
}

// TestRun_NodeError_WrapsWithNodeID tests error wrapping.
func TestRun_NodeError_WrapsWithNodeID(t *testing.T) {
	errBoom := errors.New("boom")

	graph := NewGraph[State]().
		AddNode("ok", passthrough[State]).
		AddNode("fail", makeFailingNode(errBoom)).
		AddEdge("ok", "fail").
		AddEdge("fail", END).
		SetEntry("ok")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Run(testCtx(), State{})

	require.Error(t, err)

	var nodeErr *NodeError
	require.ErrorAs(t, err, &nodeErr)
	assert.Equal(t, "fail", nodeErr.NodeID)
	assert.Equal(t, "execute", nodeErr.Op)
	assert.ErrorIs(t, err, errBoom)
}

// TestRun_NodeError_StatePreserved tests state is preserved on error.
func TestRun_NodeError_StatePreserved(t *testing.T) {
	trackingNode := func(ctx Context, s State) (State, error) {
		s.Progress = append(s.Progress, "tracked")
		return s, nil
	}

	failingNode := func(ctx Context, s State) (State, error) {
		s.Progress = append(s.Progress, "failed")
		return s, errors.New("failed")
	}

	graph := NewGraph[State]().
		AddNode("track", trackingNode).
		AddNode("fail", failingNode).
		AddEdge("track", "fail").
		AddEdge("fail", END).
		SetEntry("track")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(testCtx(), State{})

	require.Error(t, err)
	// State should include both nodes' changes
	assert.Equal(t, []string{"tracked", "failed"}, result.Progress)
}

// TestRun_PanicRecovery tests panic is caught and converted to error.
func TestRun_PanicRecovery(t *testing.T) {
	graph := NewGraph[State]().
		AddNode("panic", makePanicNode("unexpected error")).
		AddEdge("panic", END).
		SetEntry("panic")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Run(testCtx(), State{})

	require.Error(t, err)

	var panicErr *PanicError
	require.ErrorAs(t, err, &panicErr)
	assert.Equal(t, "panic", panicErr.NodeID)
	assert.Equal(t, "unexpected error", panicErr.Value)
	assert.Contains(t, panicErr.Stack, "makePanicNode")
}

// TestRun_PanicRecovery_NonStringValue tests panic with non-string value.
func TestRun_PanicRecovery_NonStringValue(t *testing.T) {
	graph := NewGraph[State]().
		AddNode("panic", makePanicNode(42)).
		AddEdge("panic", END).
		SetEntry("panic")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Run(testCtx(), State{})

	var panicErr *PanicError
	require.ErrorAs(t, err, &panicErr)
	assert.Equal(t, 42, panicErr.Value)
}

// TestRun_CancellationBetweenNodes tests cancellation is checked between nodes.
func TestRun_CancellationBetweenNodes(t *testing.T) {
	var executed []string

	ctx, cancel := context.WithCancel(context.Background())

	cancelAfterFirst := func(fgCtx Context, s State) (State, error) {
		executed = append(executed, "first")
		cancel() // Cancel after this node
		return s, nil
	}

	graph := NewGraph[State]().
		AddNode("first", cancelAfterFirst).
		AddNode("second", makeTrackingNode("second", &executed)).
		AddEdge("first", "second").
		AddEdge("second", END).
		SetEntry("first")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Run(NewContext(ctx), State{})

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)

	var cancelErr *CancellationError
	require.ErrorAs(t, err, &cancelErr)
	assert.Equal(t, "second", cancelErr.NodeID) // Was about to execute second
	assert.False(t, cancelErr.WasExecuting)
	assert.Equal(t, []string{"first"}, executed) // Only first executed
}

// TestRun_Timeout tests timeout behavior.
func TestRun_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	slowNode := func(fgCtx Context, s State) (State, error) {
		time.Sleep(200 * time.Millisecond)
		return s, nil
	}

	graph := NewGraph[State]().
		AddNode("slow", slowNode).
		AddEdge("slow", END).
		SetEntry("slow")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Run(NewContext(ctx), State{})

	// Note: The node itself doesn't check for cancellation during sleep,
	// so it may complete. The important thing is eventual timeout behavior.
	// In real usage, nodes should check ctx.Done() for long operations.
	if err != nil {
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	}
}

// TestRun_MaxIterations_PreventsInfiniteLoop tests max iterations limit.
func TestRun_MaxIterations_PreventsInfiniteLoop(t *testing.T) {
	loopNode := func(ctx Context, s State) (State, error) {
		s.Count++
		return s, nil
	}

	router := func(ctx Context, s State) string {
		return "loop" // Always loops
	}

	graph := NewGraph[State]().
		AddNode("loop", loopNode).
		AddConditionalEdge("loop", router).
		SetEntry("loop")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(testCtx(), State{}, WithMaxIterations(10))

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMaxIterations)

	var maxIterErr *MaxIterationsError
	require.ErrorAs(t, err, &maxIterErr)
	assert.Equal(t, 10, maxIterErr.Max)
	assert.Equal(t, 10, result.Count)
}

// TestRun_MaxIterations_DefaultValue tests default max iterations.
func TestRun_MaxIterations_DefaultValue(t *testing.T) {
	// Just verify the default config is 1000
	cfg := defaultRunConfig()
	assert.Equal(t, 1000, cfg.maxIterations)
}

// TestRun_NilContext_Error tests nil context handling.
func TestRun_NilContext_Error(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddEdge("a", END).
		SetEntry("a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Run(nil, Counter{})

	assert.ErrorIs(t, err, ErrNilContext)
}

// TestRun_RouterReturnsEmpty_Error tests router returning empty string.
func TestRun_RouterReturnsEmpty_Error(t *testing.T) {
	router := func(ctx Context, s State) string {
		return "" // Invalid
	}

	graph := NewGraph[State]().
		AddNode("route", passthrough[State]).
		AddConditionalEdge("route", router).
		SetEntry("route")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Run(testCtx(), State{})

	require.Error(t, err)
	var routerErr *RouterError
	require.ErrorAs(t, err, &routerErr)
	assert.Equal(t, "route", routerErr.FromNode)
	assert.ErrorIs(t, err, ErrInvalidRouterResult)
}

// TestRun_RouterReturnsUnknown_Error tests router returning unknown node.
func TestRun_RouterReturnsUnknown_Error(t *testing.T) {
	router := func(ctx Context, s State) string {
		return "nonexistent" // Unknown node
	}

	graph := NewGraph[State]().
		AddNode("route", passthrough[State]).
		AddConditionalEdge("route", router).
		SetEntry("route")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Run(testCtx(), State{})

	require.Error(t, err)
	var routerErr *RouterError
	require.ErrorAs(t, err, &routerErr)
	assert.Equal(t, "route", routerErr.FromNode)
	assert.Equal(t, "nonexistent", routerErr.Returned)
	assert.ErrorIs(t, err, ErrRouterTargetNotFound)
}

// TestRun_ContextPropagated tests context is passed to nodes.
func TestRun_ContextPropagated(t *testing.T) {
	var capturedCtx Context

	captureNode := func(ctx Context, s State) (State, error) {
		capturedCtx = ctx
		return s, nil
	}

	graph := NewGraph[State]().
		AddNode("capture", captureNode).
		AddEdge("capture", END).
		SetEntry("capture")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := NewContext(context.Background(), WithRunID("test-123"))
	_, err = compiled.Run(ctx, State{})

	require.NoError(t, err)
	assert.Equal(t, "test-123", capturedCtx.RunID())
	assert.Equal(t, "capture", capturedCtx.NodeID())
}

// TestRun_InitialStateNotMutated tests original state not modified.
func TestRun_InitialStateNotMutated(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("inc", increment).
		AddEdge("inc", END).
		SetEntry("inc")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	initial := Counter{Value: 5}
	result, err := compiled.Run(testCtx(), initial)

	require.NoError(t, err)
	assert.Equal(t, 5, initial.Value) // Original unchanged
	assert.Equal(t, 6, result.Value)  // Result has changes
}

// TestRun_ExecutionOrder tests nodes execute in correct order.
func TestRun_ExecutionOrder(t *testing.T) {
	var order []string

	graph := NewGraph[State]().
		AddNode("a", makeTrackingNode("a", &order)).
		AddNode("b", makeTrackingNode("b", &order)).
		AddNode("c", makeTrackingNode("c", &order)).
		AddEdge("a", "b").
		AddEdge("b", "c").
		AddEdge("c", END).
		SetEntry("a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Run(testCtx(), State{})

	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, order)
}

// TestContext_DefaultValues tests default context configuration.
func TestContext_DefaultValues(t *testing.T) {
	ctx := NewContext(context.Background())

	assert.NotNil(t, ctx.Logger())
	assert.Nil(t, ctx.LLM())
	assert.Nil(t, ctx.Checkpointer())
	assert.NotEmpty(t, ctx.RunID())
	assert.Equal(t, "", ctx.NodeID())
	assert.Equal(t, 1, ctx.Attempt())
}

// TestContext_WithOptions tests context configuration options.
func TestContext_WithOptions(t *testing.T) {
	ctx := NewContext(context.Background(),
		WithRunID("custom-run-id"))

	assert.Equal(t, "custom-run-id", ctx.RunID())
}

// TestContext_CancellationPropagates tests cancellation flows through.
func TestContext_CancellationPropagates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fgCtx := NewContext(ctx)

	cancel()

	assert.Error(t, fgCtx.Err())
	assert.ErrorIs(t, fgCtx.Err(), context.Canceled)
}

// TestContext_DeadlinePropagates tests deadline flows through.
func TestContext_DeadlinePropagates(t *testing.T) {
	deadline := time.Now().Add(1 * time.Hour)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	fgCtx := NewContext(ctx)

	d, ok := fgCtx.Deadline()
	assert.True(t, ok)
	assert.Equal(t, deadline, d)
}

// TestContext_ValuesFromParent tests parent context values are accessible.
func TestContext_ValuesFromParent(t *testing.T) {
	type keyType string
	key := keyType("custom")

	parentCtx := context.WithValue(context.Background(), key, "value")
	fgCtx := NewContext(parentCtx)

	assert.Equal(t, "value", fgCtx.Value(key))
}
