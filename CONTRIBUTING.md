# Contributing to flowgraph

Thank you for your interest in contributing to flowgraph!

## Development Setup

### Prerequisites

- Go 1.22 or later
- Git

### Clone and Build

```bash
git clone https://github.com/rmurphy/flowgraph.git
cd flowgraph
go build ./...
```

### Run Tests

```bash
# Run all tests with race detection
go test -race ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out  # Open in browser
```

### Run Benchmarks

```bash
go test -bench=. ./benchmarks/...
```

## Code Style

### Formatting

Always run `gofmt` before committing:

```bash
gofmt -s -w .
```

### Linting

Run `go vet` to check for common issues:

```bash
go vet ./...
```

### Imports

Use `goimports` to organize imports:

```bash
go install golang.org/x/tools/cmd/goimports@latest
goimports -w .
```

## Documentation

### Godoc

- Every exported type, function, and constant must have a doc comment
- Doc comments should be complete sentences starting with the name of the element
- Include usage examples in doc comments where helpful

Example:
```go
// NewGraph creates a new graph builder for state type S.
// Use the fluent API to add nodes and edges, then call Compile().
//
// Example:
//
//	graph := flowgraph.NewGraph[State]().
//	    AddNode("process", processNode).
//	    AddEdge("process", flowgraph.END).
//	    SetEntry("process")
func NewGraph[S any]() *Graph[S] {
    // ...
}
```

### Examples

Each example in `examples/` should:
- Be a complete, runnable `main.go`
- Have a `README.md` explaining what it demonstrates
- Include helpful comments

## Testing

### Test Requirements

- Target 85%+ coverage for core packages
- Include both success and error path tests
- Use table-driven tests for multiple scenarios
- Test with race detection (`-race` flag)

### Test Organization

- Unit tests: Same package, `*_test.go` files
- Internal tests: `internal_test.go` for testing private functions
- External tests: `*_test` package for testing public API

### Mock Objects

Use the provided `MockClient` for LLM testing:

```go
mock := llm.NewMockClient("response")
ctx := flowgraph.NewContext(context.Background(), flowgraph.WithLLM(mock))
```

## Pull Request Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Ensure all tests pass (`go test -race ./...`)
5. Run formatting and linting (`gofmt -s -w . && go vet ./...`)
6. Commit your changes with a clear message
7. Push to your fork
8. Open a Pull Request

### PR Guidelines

- Keep PRs focused on a single feature or fix
- Include tests for new functionality
- Update documentation as needed
- Reference any related issues

### Commit Messages

Use clear, descriptive commit messages:

```
Add checkpointing support for crash recovery

- Add CheckpointStore interface
- Implement SQLiteStore for persistent storage
- Add WithCheckpointing run option
- Add Resume and ResumeFrom methods
```

## Reporting Issues

When reporting issues, please include:

- Go version (`go version`)
- flowgraph version (commit hash or tag)
- Minimal code to reproduce the issue
- Expected behavior
- Actual behavior
- Error messages or stack traces

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and improve

## Questions?

Feel free to open an issue for questions about contributing or using flowgraph.
