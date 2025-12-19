# Loop/Retry Example

This example demonstrates a retry pattern using conditional edges that loop back to earlier nodes.

## What It Shows

- Creating loops with conditional edges
- Tracking attempts in state
- Breaking out of loops based on conditions
- Max iterations protection (built-in)

## Graph Structure

```
          ┌─────────┐
          │ attempt │ ◄───┐
          └────┬────┘     │
               │          │
          [router]        │
         /         \      │
    success    !success   │
       │           │      │
       │      attempts<3 ─┘
       │           │
       │      attempts>=3
       │           │
       └─────┬─────┘
             │
       ┌─────┴─────┐
       │   done    │
       └─────┬─────┘
             │
           [END]
```

## Running

```bash
go run main.go
```

## Example Output

```
=== Run 1 ===
Attempt 1: Failed, will retry...
Attempt 2: Failed, will retry...
Attempt 3: Success!
Final: Completed successfully after 3 attempt(s)
Final state: success=true, attempts=3

=== Run 2 ===
Attempt 1: Success!
Final: Completed successfully after 1 attempt(s)
Final state: success=true, attempts=1

=== Run 3 ===
Attempt 1: Failed, will retry...
Attempt 2: Failed, will retry...
Attempt 3: Failed, will retry...
Final: Failed after 3 attempts, giving up
Final state: success=false, attempts=3
```

## Key Concepts

1. **Conditional loops**: Router returns the current node ID to loop
2. **State-based exit**: Use state fields to track and limit iterations
3. **Built-in protection**: flowgraph has a default max iterations (100) to prevent infinite loops
4. **Clean exit**: Always have a path to END to avoid hanging
