/*
Package flowgraph provides graph-based orchestration for LLM workflows.

# Overview

flowgraph is a Go library for building and executing directed graphs
where nodes perform work and edges define flow. It's designed for
orchestrating LLM-powered workflows with features like checkpointing,
conditional branching, and crash recovery.

The library is inspired by LangGraph but built for Go with:
  - Type-safe generics for state management
  - Compile-time validation of graph structure
  - Built-in crash recovery via checkpointing
  - OpenTelemetry integration for observability

# Basic Usage

Create a graph with nodes and edges, then compile and run:

	type State struct {
	    Input  string
	    Output string
	}

	func process(ctx flowgraph.Context, s State) (State, error) {
	    s.Output = "Processed: " + s.Input
	    return s, nil
	}

	func main() {
	    graph := flowgraph.NewGraph[State]().
	        AddNode("process", process).
	        AddEdge("process", flowgraph.END).
	        SetEntry("process")

	    compiled, err := graph.Compile()
	    if err != nil {
	        log.Fatal(err)
	    }

	    ctx := flowgraph.NewContext(context.Background())
	    result, err := compiled.Run(ctx, State{Input: "hello"})
	    if err != nil {
	        log.Fatal(err)
	    }
	    fmt.Println(result.Output) // "Processed: hello"
	}

# Conditional Branching

Use conditional edges for decision points:

	graph.AddConditionalEdge("review", func(ctx flowgraph.Context, s State) string {
	    if s.Approved {
	        return "publish"
	    }
	    return "revise"
	})

The router function returns the ID of the next node to execute.
Invalid return values (referencing non-existent nodes) cause runtime errors.

# Loops

Create loops by having conditional edges that return to earlier nodes:

	graph := flowgraph.NewGraph[RetryState]().
	    AddNode("attempt", tryOperation).
	    AddNode("cleanup", cleanupOnSuccess).
	    AddConditionalEdge("attempt", func(ctx flowgraph.Context, s RetryState) string {
	        if s.Success || s.Attempts >= 3 {
	            return "cleanup"
	        }
	        return "attempt" // Loop back
	    }).
	    AddEdge("cleanup", flowgraph.END).
	    SetEntry("attempt")

Loops are protected by max iterations (default 100) to prevent infinite loops.
Configure with WithMaxIterations option.

# Checkpointing

Enable crash recovery with checkpointing:

	store := checkpoint.NewSQLiteStore("./checkpoints.db")
	defer store.Close()

	result, err := compiled.Run(ctx, state,
	    flowgraph.WithCheckpointing(store),
	    flowgraph.WithRunID("run-123"))

	// Resume after crash
	result, err = compiled.Resume(ctx, store, "run-123")

Checkpoints are saved after each successful node execution.
When resuming, execution continues from the node after the last checkpoint.

# LLM Integration

Use the LLM client interface for AI calls within nodes:

	func generateSpec(ctx flowgraph.Context, s State) (State, error) {
	    resp, err := ctx.LLM().Complete(ctx, llm.CompletionRequest{
	        Messages: []llm.Message{{Role: llm.RoleUser, Content: s.Input}},
	    })
	    if err != nil {
	        return s, err
	    }
	    s.Output = resp.Content
	    return s, nil
	}

	client := llm.NewClaudeCLI(
	    llm.WithModel("sonnet"),
	    llm.WithDangerouslySkipPermissions(),
	    llm.WithMaxBudgetUSD(1.0),
	)
	ctx := flowgraph.NewContext(context.Background(), flowgraph.WithLLM(client))

The Claude CLI client supports JSON output with full token tracking,
session management, and budget controls.

# Observability

Enable logging, metrics, and tracing:

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	result, err := compiled.Run(ctx, state,
	    flowgraph.WithObservabilityLogger(logger),
	    flowgraph.WithMetrics(true),
	    flowgraph.WithTracing(true),
	    flowgraph.WithRunID("run-123"))

Logs include structured fields: run_id, node_id, duration_ms, attempt.
OpenTelemetry metrics: flowgraph.node.executions, flowgraph.node.latency_ms, etc.
OpenTelemetry tracing: flowgraph.run > flowgraph.node.{id} spans.

# Error Handling

Errors include context about which node failed:

	result, err := compiled.Run(ctx, state)
	var nodeErr *flowgraph.NodeError
	if errors.As(err, &nodeErr) {
	    log.Printf("Node %s failed: %v", nodeErr.NodeID, nodeErr.Err)
	}

	var panicErr *flowgraph.PanicError
	if errors.As(err, &panicErr) {
	    log.Printf("Node %s panicked: %v\n%s", panicErr.NodeID, panicErr.Value, panicErr.Stack)
	}

Panics in nodes are recovered and converted to PanicError with stack trace.

# Thread Safety

  - Graph[S] is NOT safe for concurrent use during construction
  - CompiledGraph[S] IS safe for concurrent use (immutable)
  - Context IS safe for concurrent use
  - CheckpointStore implementations are safe for concurrent use

# Subpackages

  - checkpoint: Checkpoint storage (memory, SQLite)
  - llm: LLM client interface and implementations
  - observability: Logging, metrics, and tracing helpers
*/
package flowgraph
