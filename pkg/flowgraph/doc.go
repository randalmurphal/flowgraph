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

Loops are protected by max iterations (default 1000) to prevent infinite loops.
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

Use LLM clients via Go's context.WithValue pattern:

	// Context key for LLM client injection
	type llmKey struct{}
	func WithLLM(ctx context.Context, c claude.Client) context.Context {
	    return context.WithValue(ctx, llmKey{}, c)
	}
	func LLM(ctx context.Context) claude.Client {
	    if c, ok := ctx.Value(llmKey{}).(claude.Client); ok { return c }
	    return nil
	}

	// In a node:
	func generateSpec(ctx flowgraph.Context, s State) (State, error) {
	    client := LLM(ctx) // Access via context.Value
	    if client == nil {
	        return s, fmt.Errorf("LLM client not configured")
	    }
	    resp, err := client.Complete(ctx, claude.CompletionRequest{
	        Messages: []claude.Message{{Role: claude.RoleUser, Content: s.Input}},
	    })
	    if err != nil {
	        return s, err
	    }
	    s.Output = resp.Content
	    return s, nil
	}

	// Configure client and inject via context
	client := claude.NewClaudeCLI(...)
	baseCtx := WithLLM(context.Background(), client)
	ctx := flowgraph.NewContext(baseCtx)

This keeps flowgraph decoupled from specific LLM implementations while
supporting Claude CLI with JSON output, token tracking, and budget controls.

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
