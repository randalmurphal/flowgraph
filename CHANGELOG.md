# Changelog

All notable changes to flowgraph are documented in this file.

## [Unreleased]

### Added

- **Core Graph Engine** (Phase 1)
  - Type-safe generic graphs with `Graph[S]` and `CompiledGraph[S]`
  - Fluent builder API: `AddNode`, `AddEdge`, `SetEntry`
  - `END` constant for terminal nodes
  - Compile-time graph validation
  - `NodeFunc[S]` and `RouterFunc[S]` function types

- **Conditional Branching** (Phase 2)
  - `AddConditionalEdge` for runtime routing based on state
  - Router functions that return next node ID
  - Loop support with max iterations protection (default 100)

- **Checkpointing** (Phase 3)
  - `CheckpointStore` interface for pluggable storage
  - `MemoryStore` for testing and ephemeral workflows
  - `SQLiteStore` for production crash recovery
  - `Resume()` and `ResumeFrom()` methods
  - `WithCheckpointing()` and `WithRunID()` run options

- **LLM Integration** (Phase 4)
  - `Client` interface with `Complete` and `Stream` methods
  - `ClaudeCLI` implementation with full Claude CLI support
  - `MockClient` for testing with configurable responses
  - JSON output parsing with token/cost tracking
  - Session management: `WithSessionID`, `WithContinue`, `WithResume`
  - Tool control: `WithAllowedTools`, `WithDisallowedTools`
  - Budget control: `WithMaxBudgetUSD`, `WithMaxTurns`
  - Permission modes: `WithDangerouslySkipPermissions`, `WithPermissionMode`
  - System prompts: `WithSystemPrompt`, `WithAppendSystemPrompt`
  - Streaming support with `StreamChunk` channel

- **Observability** (Phase 5)
  - Structured logging via `slog` with `WithObservabilityLogger`
  - OpenTelemetry metrics: node executions, latency, errors
  - OpenTelemetry tracing: run and node spans
  - No-op implementations for disabled observability
  - Log enrichment helpers in `observability` package

- **Error Handling**
  - `NodeError` wrapping errors with node context
  - `PanicError` with recovered value and stack trace
  - Sentinel errors: `ErrNoEntryPoint`, `ErrNoPathToEnd`, etc.
  - Retryable error detection for LLM rate limits

- **Context**
  - Custom `Context` interface wrapping `context.Context`
  - `WithLLM()` for injecting LLM client
  - Thread-safe concurrent access

### Fixed

- Conditional edge reachability analysis now correctly marks all nodes as potentially reachable when a conditional edge is present, eliminating spurious "unreachable node" warnings

### Performance

- Per-node execution overhead: < 1μs
- Context creation: < 100ns
- SQLite checkpoint save: < 1ms
- Memory checkpoint: < 400ns

## [0.1.0] - Initial Release

Initial implementation with core features for graph-based LLM orchestration.
