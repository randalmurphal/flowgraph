package flowgraph

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// TestState is a simple state for parallel tests
type TestState struct {
	Values   map[string]int
	BranchID string
}

func (s TestState) Clone(branchID string) TestState {
	clone := TestState{
		Values:   make(map[string]int),
		BranchID: branchID,
	}
	for k, v := range s.Values {
		clone.Values[k] = v
	}
	return clone
}

func (s TestState) Merge(branches map[string]TestState) TestState {
	merged := TestState{
		Values: make(map[string]int),
	}
	// Copy original values
	for k, v := range s.Values {
		merged.Values[k] = v
	}
	// Add branch results
	for branchID, branchState := range branches {
		for k, v := range branchState.Values {
			merged.Values[branchID+"_"+k] = v
		}
	}
	return merged
}

func TestForkJoin_Basic(t *testing.T) {
	// Build a graph with fork/join:
	//
	//          ┌─> workerA ─┐
	// start ─> dispatch ─────┼─> collect ─> END
	//          └─> workerB ─┘

	graph := NewGraph[TestState]().
		AddNode("start", func(ctx Context, s TestState) (TestState, error) {
			s.Values["started"] = 1
			return s, nil
		}).
		AddNode("dispatch", func(ctx Context, s TestState) (TestState, error) {
			s.Values["dispatched"] = 1
			return s, nil
		}).
		AddNode("workerA", func(ctx Context, s TestState) (TestState, error) {
			s.Values["workerA_done"] = 1
			return s, nil
		}).
		AddNode("workerB", func(ctx Context, s TestState) (TestState, error) {
			s.Values["workerB_done"] = 1
			return s, nil
		}).
		AddNode("collect", func(ctx Context, s TestState) (TestState, error) {
			s.Values["collected"] = 1
			return s, nil
		}).
		AddEdge("start", "dispatch").
		AddEdge("dispatch", "workerA").
		AddEdge("dispatch", "workerB").
		AddEdge("workerA", "collect").
		AddEdge("workerB", "collect").
		AddEdge("collect", END).
		SetEntry("start")

	compiled, err := graph.Compile()
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}

	// Verify fork/join detection
	if !compiled.HasParallelExecution() {
		t.Error("Expected parallel execution to be detected")
	}

	fork := compiled.GetForkNode("dispatch")
	if fork == nil {
		t.Fatal("dispatch should be detected as fork node")
	}
	if fork.JoinNodeID != "collect" {
		t.Errorf("Expected join node 'collect', got %q", fork.JoinNodeID)
	}
	if len(fork.Branches) != 2 {
		t.Errorf("Expected 2 branches, got %d", len(fork.Branches))
	}

	// Execute the graph
	ctx := NewContext(context.Background())
	initial := TestState{Values: make(map[string]int)}

	result, runErr := compiled.Run(ctx, initial)
	if runErr != nil {
		t.Fatalf("Run() error: %v", runErr)
	}

	// Verify all nodes executed
	if result.Values["started"] != 1 {
		t.Error("start node should have executed")
	}
	if result.Values["dispatched"] != 1 {
		t.Error("dispatch node should have executed")
	}
	if result.Values["collected"] != 1 {
		t.Error("collect node should have executed")
	}

	// Verify branch results were merged
	// The merge should have prefixed branch results
	if result.Values["workerA_workerA_done"] != 1 && result.Values["workerB_workerA_done"] != 1 {
		t.Error("workerA results should be merged")
	}
	if result.Values["workerA_workerB_done"] != 1 && result.Values["workerB_workerB_done"] != 1 {
		t.Error("workerB results should be merged")
	}
}

func TestForkJoin_Concurrency(t *testing.T) {
	// Verify branches execute in parallel
	var executing int32
	var maxConcurrent int32

	graph := NewGraph[TestState]().
		AddNode("start", func(ctx Context, s TestState) (TestState, error) {
			return s, nil
		}).
		AddNode("workerA", func(ctx Context, s TestState) (TestState, error) {
			current := atomic.AddInt32(&executing, 1)
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if current > max {
					atomic.CompareAndSwapInt32(&maxConcurrent, max, current)
				} else {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&executing, -1)
			return s, nil
		}).
		AddNode("workerB", func(ctx Context, s TestState) (TestState, error) {
			current := atomic.AddInt32(&executing, 1)
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if current > max {
					atomic.CompareAndSwapInt32(&maxConcurrent, max, current)
				} else {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&executing, -1)
			return s, nil
		}).
		AddNode("collect", func(ctx Context, s TestState) (TestState, error) {
			return s, nil
		}).
		AddEdge("start", "workerA").
		AddEdge("start", "workerB").
		AddEdge("workerA", "collect").
		AddEdge("workerB", "collect").
		AddEdge("collect", END).
		SetEntry("start")

	compiled, err := graph.Compile()
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}

	ctx := NewContext(context.Background())
	initial := TestState{Values: make(map[string]int)}

	startTime := time.Now()
	_, runErr := compiled.Run(ctx, initial)
	duration := time.Since(startTime)

	if runErr != nil {
		t.Fatalf("Run() error: %v", runErr)
	}

	// If executed in parallel, max concurrent should be 2
	if atomic.LoadInt32(&maxConcurrent) < 2 {
		t.Errorf("Expected concurrent execution, but max concurrent was %d", maxConcurrent)
	}

	// If executed in parallel, duration should be around 50ms, not 100ms
	if duration > 80*time.Millisecond {
		t.Errorf("Expected parallel execution to complete in ~50ms, took %v", duration)
	}
}

func TestForkJoin_BranchError(t *testing.T) {
	graph := NewGraph[TestState]().
		AddNode("start", func(ctx Context, s TestState) (TestState, error) {
			return s, nil
		}).
		AddNode("workerA", func(ctx Context, s TestState) (TestState, error) {
			return s, nil
		}).
		AddNode("workerB", func(ctx Context, s TestState) (TestState, error) {
			return s, fmt.Errorf("workerB failed")
		}).
		AddNode("collect", func(ctx Context, s TestState) (TestState, error) {
			return s, nil
		}).
		AddEdge("start", "workerA").
		AddEdge("start", "workerB").
		AddEdge("workerA", "collect").
		AddEdge("workerB", "collect").
		AddEdge("collect", END).
		SetEntry("start")

	compiled, err := graph.Compile()
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}

	ctx := NewContext(context.Background())
	initial := TestState{Values: make(map[string]int)}

	_, runErr := compiled.Run(ctx, initial)
	if runErr == nil {
		t.Fatal("Expected error from failed branch")
	}

	// Should be a ForkJoinError
	var forkErr *ForkJoinError
	if !containsError(runErr, &forkErr) {
		t.Errorf("Expected ForkJoinError, got %T: %v", runErr, runErr)
	}
}

func TestForkJoin_WithBranchHook(t *testing.T) {
	var onForkCalls []string
	var onJoinCalled bool
	var onJoinBranches []string

	hook := &testBranchHook{
		onFork: func(ctx Context, branchID string, s TestState) (TestState, error) {
			onForkCalls = append(onForkCalls, branchID)
			s.Values["hook_"+branchID] = 1
			return s, nil
		},
		onJoin: func(ctx Context, branchStates map[string]TestState) error {
			onJoinCalled = true
			for branchID := range branchStates {
				onJoinBranches = append(onJoinBranches, branchID)
			}
			return nil
		},
	}

	graph := NewGraph[TestState]().
		AddNode("start", func(ctx Context, s TestState) (TestState, error) {
			return s, nil
		}).
		AddNode("workerA", func(ctx Context, s TestState) (TestState, error) {
			if s.Values["hook_workerA"] != 1 {
				return s, fmt.Errorf("hook not called for workerA")
			}
			return s, nil
		}).
		AddNode("workerB", func(ctx Context, s TestState) (TestState, error) {
			if s.Values["hook_workerB"] != 1 {
				return s, fmt.Errorf("hook not called for workerB")
			}
			return s, nil
		}).
		AddNode("collect", func(ctx Context, s TestState) (TestState, error) {
			return s, nil
		}).
		AddEdge("start", "workerA").
		AddEdge("start", "workerB").
		AddEdge("workerA", "collect").
		AddEdge("workerB", "collect").
		AddEdge("collect", END).
		SetEntry("start").
		SetBranchHook(hook)

	compiled, err := graph.Compile()
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}

	ctx := NewContext(context.Background())
	initial := TestState{Values: make(map[string]int)}

	_, runErr := compiled.Run(ctx, initial)
	if runErr != nil {
		t.Fatalf("Run() error: %v", runErr)
	}

	// Verify OnFork was called for each branch
	if len(onForkCalls) != 2 {
		t.Errorf("Expected 2 OnFork calls, got %d", len(onForkCalls))
	}

	// Verify OnJoin was called
	if !onJoinCalled {
		t.Error("OnJoin should have been called")
	}
	if len(onJoinBranches) != 2 {
		t.Errorf("Expected 2 branches in OnJoin, got %d", len(onJoinBranches))
	}
}

func TestForkJoin_MaxConcurrency(t *testing.T) {
	var executing int32
	var maxConcurrent int32

	graph := NewGraph[TestState]().
		AddNode("start", func(ctx Context, s TestState) (TestState, error) {
			return s, nil
		}).
		AddNode("workerA", func(ctx Context, s TestState) (TestState, error) {
			current := atomic.AddInt32(&executing, 1)
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if current > max {
					atomic.CompareAndSwapInt32(&maxConcurrent, max, current)
				} else {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&executing, -1)
			return s, nil
		}).
		AddNode("workerB", func(ctx Context, s TestState) (TestState, error) {
			current := atomic.AddInt32(&executing, 1)
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if current > max {
					atomic.CompareAndSwapInt32(&maxConcurrent, max, current)
				} else {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&executing, -1)
			return s, nil
		}).
		AddNode("workerC", func(ctx Context, s TestState) (TestState, error) {
			current := atomic.AddInt32(&executing, 1)
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if current > max {
					atomic.CompareAndSwapInt32(&maxConcurrent, max, current)
				} else {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&executing, -1)
			return s, nil
		}).
		AddNode("collect", func(ctx Context, s TestState) (TestState, error) {
			return s, nil
		}).
		AddEdge("start", "workerA").
		AddEdge("start", "workerB").
		AddEdge("start", "workerC").
		AddEdge("workerA", "collect").
		AddEdge("workerB", "collect").
		AddEdge("workerC", "collect").
		AddEdge("collect", END).
		SetEntry("start").
		SetForkJoinConfig(ForkJoinConfig{
			MaxConcurrency: 2, // Limit to 2 concurrent
		})

	compiled, err := graph.Compile()
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}

	ctx := NewContext(context.Background())
	initial := TestState{Values: make(map[string]int)}

	_, runErr := compiled.Run(ctx, initial)
	if runErr != nil {
		t.Fatalf("Run() error: %v", runErr)
	}

	// With MaxConcurrency=2, we should never exceed 2 concurrent workers
	if atomic.LoadInt32(&maxConcurrent) > 2 {
		t.Errorf("Expected max 2 concurrent, but got %d", maxConcurrent)
	}
}

func TestNoForkJoin_SequentialExecution(t *testing.T) {
	// Verify that graphs without fork/join still work
	graph := NewGraph[TestState]().
		AddNode("a", func(ctx Context, s TestState) (TestState, error) {
			s.Values["a"] = 1
			return s, nil
		}).
		AddNode("b", func(ctx Context, s TestState) (TestState, error) {
			s.Values["b"] = 1
			return s, nil
		}).
		AddEdge("a", "b").
		AddEdge("b", END).
		SetEntry("a")

	compiled, err := graph.Compile()
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}

	if compiled.HasParallelExecution() {
		t.Error("Should not detect parallel execution in linear graph")
	}

	ctx := NewContext(context.Background())
	initial := TestState{Values: make(map[string]int)}

	result, runErr := compiled.Run(ctx, initial)
	if runErr != nil {
		t.Fatalf("Run() error: %v", runErr)
	}

	if result.Values["a"] != 1 || result.Values["b"] != 1 {
		t.Error("Sequential execution should work normally")
	}
}

// testBranchHook is a test implementation of BranchHook
type testBranchHook struct {
	onFork        func(Context, string, TestState) (TestState, error)
	onJoin        func(Context, map[string]TestState) error
	onBranchError func(Context, string, TestState, error)
}

func (h *testBranchHook) OnFork(ctx Context, branchID string, s TestState) (TestState, error) {
	if h.onFork != nil {
		return h.onFork(ctx, branchID, s)
	}
	return s, nil
}

func (h *testBranchHook) OnJoin(ctx Context, branchStates map[string]TestState) error {
	if h.onJoin != nil {
		return h.onJoin(ctx, branchStates)
	}
	return nil
}

func (h *testBranchHook) OnBranchError(ctx Context, branchID string, s TestState, err error) {
	if h.onBranchError != nil {
		h.onBranchError(ctx, branchID, s, err)
	}
}

// containsError checks if err or any wrapped error matches the target type
func containsError[T error](err error, target *T) bool {
	for err != nil {
		if _, ok := err.(T); ok {
			return true
		}
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
		} else {
			break
		}
	}
	return false
}
