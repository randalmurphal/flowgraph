# ADR-024: Tracing Integration

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

How should flowgraph integrate with distributed tracing? Tracing is essential for:
- Understanding execution flow
- Debugging production issues
- Performance analysis
- Correlating with external services

## Decision

**OpenTelemetry tracing with automatic span creation per node.**

### Approach

```go
import "go.opentelemetry.io/otel/trace"

// flowgraph creates spans for graph execution
func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...RunOption) (S, error) {
    tracer := otel.Tracer("flowgraph")

    // Root span for entire run
    ctx, span := tracer.Start(ctx, "flowgraph.run",
        trace.WithAttributes(
            attribute.String("graph.entry", cg.entryPoint),
            attribute.String("run_id", ctx.RunID()),
        ),
    )
    defer span.End()

    current := cg.entryPoint
    for current != END {
        state, err = cg.executeNodeWithTracing(ctx, tracer, current, state)
        if err != nil {
            span.RecordError(err)
            span.SetStatus(codes.Error, err.Error())
            return state, err
        }
        current, _ = cg.nextNode(ctx, state, current)
    }

    span.SetStatus(codes.Ok, "")
    return state, nil
}

func (cg *CompiledGraph[S]) executeNodeWithTracing(
    ctx Context,
    tracer trace.Tracer,
    nodeID string,
    state S,
) (S, error) {
    ctx, span := tracer.Start(ctx, "flowgraph.node."+nodeID,
        trace.WithAttributes(
            attribute.String("node.id", nodeID),
        ),
    )
    defer span.End()

    result, err := cg.executeNode(ctx, nodeID, state)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }

    return result, err
}
```

### Trace Hierarchy

```
flowgraph.run (run_id=abc123)
├── flowgraph.node.parse-ticket
├── flowgraph.node.generate-spec
│   └── llm.complete (prompt_length=500, tokens_in=500, tokens_out=2000)
├── flowgraph.node.implement
│   └── llm.complete
├── flowgraph.node.review
│   └── llm.complete
└── flowgraph.node.create-pr
    └── git.commit
```

## Alternatives Considered

### 1. No Built-in Tracing

```go
// Users create spans in their nodes
func myNode(ctx flowgraph.Context, state State) (State, error) {
    ctx, span := otel.Tracer("myapp").Start(ctx, "myNode")
    defer span.End()
    // ...
}
```

**Rejected**: Misses graph-level spans. Requires every node to do this.

### 2. Hooks for Tracing

```go
graph.Run(ctx, state,
    WithNodeHooks(
        func(nodeID string) { startSpan(nodeID) },
        func(nodeID string, err error) { endSpan(nodeID, err) },
    ),
)
```

**Rejected**: Hooks don't have access to context propagation.

### 3. Custom Tracing Interface

```go
type Tracer interface {
    StartSpan(name string) Span
}
```

**Rejected**: Would need to wrap OTel. Just use OTel directly.

## Consequences

### Positive
- **Automatic** - Spans created without user code
- **Standard** - OpenTelemetry works with all tracing backends
- **Rich** - Full execution flow visible
- **Context propagation** - Works across services

### Negative
- OTel dependency
- Small overhead per span (~microseconds)

### Risks
- Too many spans for large graphs → Can disable per-node spans

---

## Span Attributes

### Run Span
| Attribute | Type | Description |
|-----------|------|-------------|
| graph.entry | string | Entry point node |
| run_id | string | Unique run identifier |
| graph.name | string | Optional graph name |

### Node Span
| Attribute | Type | Description |
|-----------|------|-------------|
| node.id | string | Node identifier |
| node.attempt | int | Attempt number (for retries) |
| node.checkpoint | bool | Whether checkpointing enabled |

### LLM Span (if integrated)
| Attribute | Type | Description |
|-----------|------|-------------|
| llm.model | string | Model used |
| llm.tokens_in | int | Input tokens |
| llm.tokens_out | int | Output tokens |
| llm.duration_ms | int | LLM call duration |

### Error Span
| Attribute | Type | Description |
|-----------|------|-------------|
| exception.type | string | Error type |
| exception.message | string | Error message |
| exception.stacktrace | string | Stack trace (for panics) |

---

## Usage Examples

### Enable Tracing with Jaeger

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/trace"
)

// Setup OTLP exporter (Jaeger, etc.)
exporter, _ := otlptracegrpc.New(ctx,
    otlptracegrpc.WithEndpoint("localhost:4317"),
    otlptracegrpc.WithInsecure(),
)

provider := trace.NewTracerProvider(
    trace.WithBatcher(exporter),
    trace.WithResource(resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceName("my-orchestrator"),
    )),
)
otel.SetTracerProvider(provider)

// flowgraph automatically uses global tracer
result, err := compiled.Run(ctx, state)
```

### Tracing in Nodes

```go
func myNode(ctx flowgraph.Context, state State) (State, error) {
    // Span already created for this node
    // Add custom events/attributes
    span := trace.SpanFromContext(ctx)

    span.AddEvent("processing-started")
    span.SetAttributes(attribute.Int("input_size", len(state.Input)))

    result, err := process(ctx, state)

    span.AddEvent("processing-completed")
    return result, err
}
```

### Tracing Across Services

```go
func callExternalService(ctx context.Context, data string) error {
    // Trace context propagates automatically via ctx
    req, _ := http.NewRequestWithContext(ctx, "POST", url, body)

    // Inject trace context into headers
    otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

    return client.Do(req)
}
```

### Disable Per-Node Spans

```go
// For performance-sensitive runs
result, err := compiled.Run(ctx, state,
    flowgraph.WithTracing(flowgraph.TracingOptions{
        NodeSpans: false,  // Only create run-level span
    }),
)
```

---

## Trace Visualization

In Jaeger/Zipkin, a trace looks like:

```
┌─────────────────────────────────────────────────────────────────────┐
│ flowgraph.run                                           12.5s       │
│ run_id=abc123, entry=parse-ticket                                   │
├─────────────────────────────────────────────────────────────────────┤
│ ├─ flowgraph.node.parse-ticket                          50ms        │
│ │                                                                   │
│ ├─ flowgraph.node.generate-spec                         3.2s        │
│ │  └─ llm.complete  tokens_in=500 tokens_out=2000       3.1s        │
│ │                                                                   │
│ ├─ flowgraph.node.implement                             5.8s        │
│ │  └─ llm.complete  tokens_in=2500 tokens_out=5000      5.6s        │
│ │                                                                   │
│ ├─ flowgraph.node.review                                2.1s        │
│ │  └─ llm.complete  tokens_in=5500 tokens_out=1500      2.0s        │
│ │                                                                   │
│ └─ flowgraph.node.create-pr                             1.2s        │
│    ├─ git.add                                           100ms       │
│    ├─ git.commit                                        200ms       │
│    └─ git.push                                          800ms       │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Test Cases

```go
func TestTracing_SpanCreation(t *testing.T) {
    // In-memory exporter for testing
    exporter := tracetest.NewInMemoryExporter()
    provider := trace.NewTracerProvider(
        trace.WithSyncer(exporter),
    )
    otel.SetTracerProvider(provider)

    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("a", testNode).
        AddNode("b", testNode).
        AddEdge("a", "b").
        AddEdge("b", flowgraph.END).
        SetEntry("a").
        Compile()

    _, _ = compiled.Run(context.Background(), testState{})

    spans := exporter.GetSpans()

    // Should have: run span + 2 node spans
    assert.Len(t, spans, 3)

    // Find run span
    var runSpan tracetest.SpanStub
    for _, s := range spans {
        if s.Name == "flowgraph.run" {
            runSpan = s
            break
        }
    }
    assert.NotEmpty(t, runSpan.Name)

    // Node spans should be children of run span
    for _, s := range spans {
        if strings.HasPrefix(s.Name, "flowgraph.node.") {
            assert.Equal(t, runSpan.SpanContext.SpanID(), s.Parent.SpanID())
        }
    }
}

func TestTracing_ErrorRecording(t *testing.T) {
    exporter := tracetest.NewInMemoryExporter()
    provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
    otel.SetTracerProvider(provider)

    failingNode := func(ctx flowgraph.Context, s testState) (testState, error) {
        return s, errors.New("intentional failure")
    }

    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("fail", failingNode).
        AddEdge("fail", flowgraph.END).
        SetEntry("fail").
        Compile()

    _, _ = compiled.Run(context.Background(), testState{})

    spans := exporter.GetSpans()

    // Find failed span
    var failSpan tracetest.SpanStub
    for _, s := range spans {
        if s.Name == "flowgraph.node.fail" {
            failSpan = s
            break
        }
    }

    assert.Equal(t, codes.Error, failSpan.Status.Code)
    assert.Len(t, failSpan.Events, 1)  // Error event
    assert.Equal(t, "exception", failSpan.Events[0].Name)
}
```
