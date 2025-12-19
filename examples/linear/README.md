# Linear Flow Example

This example demonstrates basic sequential graph execution with three nodes processing state in sequence.

## What It Shows

- Creating a graph with `NewGraph[State]()`
- Adding nodes with `AddNode(id, func)`
- Connecting nodes with `AddEdge(from, to)`
- Using `flowgraph.END` as the terminal node
- Setting the entry point with `SetEntry(id)`
- Compiling and running the graph

## Graph Structure

```
step1 -> step2 -> step3 -> END
```

## Running

```bash
go run main.go
```

## Expected Output

```
Step 1: Processed: hello
Step 2: Validated: Processed: hello
Step 3: Completed: Validated: Processed: hello

Final state:
  Input:  hello
  Step1:  Processed: hello
  Step2:  Validated: Processed: hello
  Step3:  Completed: Validated: Processed: hello
```

## Key Concepts

1. **State flows through nodes**: Each node receives state, modifies it, and returns the new state
2. **Immutable state pattern**: Nodes don't modify external state, they return new state values
3. **Compile-time validation**: `Compile()` validates the graph structure before execution
4. **Sequential execution**: Nodes execute in the order defined by edges
