# Conditional Branching Example

This example demonstrates routing execution based on state values using conditional edges.

## What It Shows

- Using `AddConditionalEdge(from, routerFunc)` for branching
- Router functions that return the next node ID
- Multiple execution paths through the graph
- Same graph, different outcomes based on state

## Graph Structure

```
          ┌─────────┐
          │ review  │
          └────┬────┘
               │
          [router]
         /         \
   score >= 80    score < 80
       /               \
┌─────────┐      ┌─────────────────┐
│ approve │      │ request-changes │
└────┬────┘      └────────┬────────┘
     │                    │
     └────────┬───────────┘
              │
            [END]
```

## Running

```bash
go run main.go
```

## Expected Output

```
=== Test 1: High-quality code ===
Review: Code scored 90/100
Approve: Code approved!
Result: approved - Code meets quality standards

=== Test 2: Low-quality code ===
Review: Code scored 40/100
Request Changes: Improvements needed
Result: changes_requested - Please improve code quality
```

## Key Concepts

1. **Router functions**: Return the ID of the next node to execute
2. **Conditional edges**: Replace simple edges for decision points
3. **Multiple paths**: Different state values lead to different execution paths
4. **Type safety**: Router receives typed state, enabling compile-time checks
