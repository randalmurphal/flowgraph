# flowgraph Implementation Session

**Purpose**: Implement flowgraph Phase 6 (Polish & Documentation)

**Philosophy**: Production-ready documentation and examples. Make the library usable. No shortcuts.

---

## Context

flowgraph is a Go library for graph-based LLM workflow orchestration. Phases 1-5 are complete. Your job is to enhance the LLM client, add documentation, examples, benchmarks, and final polish.

### What's Complete

- **Phase 1**: Core graph engine - `pkg/flowgraph/*.go` (89.1% coverage)
- **Phase 2**: Conditional edges - included in Phase 1
- **Phase 3**: Checkpointing - `pkg/flowgraph/checkpoint/` (91.3% coverage)
- **Phase 4**: LLM Clients - `pkg/flowgraph/llm/` (74.7% coverage) - **NEEDS ENHANCEMENT**
- **Phase 5**: Observability - `pkg/flowgraph/observability/` (90.6% coverage)
- **27 ADRs** in `decisions/` - all architectural decisions locked
- **10 Feature Specs** in `features/` - detailed behavior specifications
- **6 Phase Specs** in `phases/` - implementation plans

### What's Ready to Build

- **Phase 6**: Polish & Documentation
  - **Step 0**: Claude CLI Integration Enhancements (LLM client is too basic)
  - **Steps 1-6**: Documentation, examples, benchmarks, godoc

---

## PRIORITY: Claude CLI Integration Enhancements

The current `ClaudeCLI` implementation is too basic. It only uses `--print` mode with text output and misses critical Claude Code CLI features needed for production use.

### Current State (What Exists)

```go
// pkg/flowgraph/llm/claude_cli.go
WithClaudePath(path)      // Binary location
WithModel(model)          // Default model
WithWorkdir(dir)          // Working directory
WithTimeout(d)            // Command timeout
WithAllowedTools(tools)   // --allowedTools flag
```

Uses `--print -p "prompt"` and returns raw text. No structured output, no token tracking, no session management.

### Gap Analysis

Run `claude --help` to see full CLI capabilities. Key missing features:

| Feature | CLI Flag | Why It Matters |
|---------|----------|----------------|
| JSON output | `--output-format json` | Get tokens, cost, session_id |
| JSON Schema | `--json-schema <schema>` | Force structured output |
| Session ID | `--session-id <uuid>` | Track/correlate runs |
| Continue/Resume | `--continue`, `--resume` | Multi-turn conversations |
| Disallowed tools | `--disallowed-tools` | Blacklist dangerous tools |
| Permission mode | `--permission-mode` | Control approval behavior |
| Skip permissions | `--dangerously-skip-permissions` | Non-interactive execution |
| Add directories | `--add-dir` | Expand file access |
| System prompt | `--system-prompt`, `--append-system-prompt` | Inject context |
| Budget limit | `--max-budget-usd` | Cap spending |
| Fallback model | `--fallback-model` | Handle overload |
| Custom agents | `--agents <json>` | Sub-agent definitions |

### Claude CLI JSON Response Format

When using `--output-format json`, Claude returns rich metadata:

```json
{
  "type": "result",
  "subtype": "success",
  "is_error": false,
  "result": "The actual response content",
  "session_id": "6de4b9fa-874e-4c4a-a7a1-d552f5b774d2",
  "duration_ms": 2695,
  "duration_api_ms": 5581,
  "num_turns": 1,
  "total_cost_usd": 0.060663,
  "usage": {
    "input_tokens": 2,
    "cache_creation_input_tokens": 8360,
    "cache_read_input_tokens": 0,
    "output_tokens": 5
  },
  "modelUsage": {
    "claude-opus-4-5-20251101": {
      "inputTokens": 2,
      "outputTokens": 5,
      "cacheReadInputTokens": 0,
      "cacheCreationInputTokens": 8360,
      "costUSD": 0.052385
    }
  }
}
```

### Implementation Plan

#### Step 0A: Core Output Improvements (~2 hours)

**Files to modify**: `pkg/flowgraph/llm/claude_cli.go`, `pkg/flowgraph/llm/request.go`

1. Add `CLIResponse` type to parse full JSON response:
```go
type CLIResponse struct {
    Type         string                   `json:"type"`
    Subtype      string                   `json:"subtype"`
    IsError      bool                     `json:"is_error"`
    Result       string                   `json:"result"`
    SessionID    string                   `json:"session_id"`
    DurationMS   int                      `json:"duration_ms"`
    NumTurns     int                      `json:"num_turns"`
    TotalCostUSD float64                  `json:"total_cost_usd"`
    Usage        UsageInfo                `json:"usage"`
    ModelUsage   map[string]ModelUsage    `json:"modelUsage"`
}

type ModelUsage struct {
    InputTokens              int     `json:"inputTokens"`
    OutputTokens             int     `json:"outputTokens"`
    CacheReadInputTokens     int     `json:"cacheReadInputTokens"`
    CacheCreationInputTokens int     `json:"cacheCreationInputTokens"`
    CostUSD                  float64 `json:"costUSD"`
}
```

2. Switch default to `--output-format json`
3. Parse full response, populate `CompletionResponse` with token usage and cost
4. Add `CostUSD` and `SessionID` fields to `CompletionResponse`

#### Step 0B: New ClaudeOption Functions (~2 hours)

Add these options to `ClaudeCLI`:

```go
// Output control
WithOutputFormat(format string)           // "text", "json", "stream-json"
WithJSONSchema(schema string)             // Force structured output

// Session management
WithSessionID(id string)                  // Use specific session
WithContinue()                            // Continue last session
WithResume(sessionID string)              // Resume specific session
WithNoSessionPersistence()                // Don't save sessions

// Tool control
WithDisallowedTools(tools []string)       // Blacklist tools
WithTools(tools []string)                 // Exact tool set

// Permissions
WithDangerouslySkipPermissions()          // Skip interactive prompts
WithPermissionMode(mode string)           // acceptEdits, bypassPermissions, etc.
WithSettingSources(sources []string)      // project, local, user

// Context
WithAddDirs(dirs []string)                // Additional directories
WithSystemPrompt(prompt string)           // Replace system prompt
WithAppendSystemPrompt(prompt string)     // Inject instructions

// Budget
WithMaxBudgetUSD(amount float64)          // Cap spending
WithFallbackModel(model string)           // Fallback on overload
```

#### Step 0C: Update buildArgs (~1 hour)

Update `buildArgs()` to include all new options in CLI command construction.

#### Step 0D: Tests (~2 hours)

- Test JSON response parsing
- Test all new options are correctly passed to CLI
- Test token/cost extraction
- Integration test with actual claude binary (skip if unavailable)

#### Step 0E: Update CompletionResponse (~1 hour)

Ensure `CompletionResponse` exposes:
- `SessionID string`
- `CostUSD float64`
- `CacheReadTokens int`
- `CacheCreationTokens int`

### Reference: Production Pattern from ensemble

See `~/repos/ai-devtools/ensemble/core/runner.py` for a battle-tested Python implementation:
- `_build_command()` - Full CLI argument construction
- `TokenUsage.from_claude_output()` - Response parsing
- `AgentConfig` - Tool restrictions, permissions, timeouts
- Error classification for retry logic

Key patterns:
- Always use `--output-format json` for structured responses
- Use `--dangerously-skip-permissions` for non-interactive
- Use `--setting-sources project,local` to avoid user config interference
- Parse `modelUsage` for per-model token breakdown

---

## Step 1: Package Documentation (~2 hours)

Create `doc.go` in `pkg/flowgraph/` with comprehensive package docs:
- Overview of flowgraph
- Basic usage example
- Conditional branching example
- Checkpointing example
- LLM integration example
- Error handling patterns

See `.spec/phases/PHASE-6-polish.md` for the full doc.go template.

---

## Step 2: README.md (~2 hours)

Update the project README with:
- Feature list
- Installation instructions
- Quick start example
- Links to examples
- Links to documentation
- Performance overview
- Contributing section

---

## Step 3: Examples (~4 hours)

Create working, copy-pasteable examples:

```
examples/
├── linear/main.go           # Simple linear flow
├── conditional/main.go      # Branching based on state
├── loop/main.go             # Retry pattern with max attempts
├── checkpointing/main.go    # Save/resume with SQLite
├── llm/main.go              # LLM integration with MockClient
└── observability/main.go    # Logging, metrics, tracing
```

Each example should:
- Be a complete, runnable main.go
- Have a README explaining what it demonstrates
- Include comments explaining the key concepts

---

## Step 4: Benchmarks (~2 hours)

Create benchmarks to establish performance baselines:

```
benchmarks/
├── graph_test.go       # NewGraph, AddNode, Compile
├── execute_test.go     # Run with various graph sizes/shapes
└── checkpoint_test.go  # Save/Load performance
```

---

## Step 5: Contributing Guide (~1 hour)

Create CONTRIBUTING.md covering:
- Development setup
- Code style (gofmt, golangci-lint)
- Testing requirements
- PR process

---

## Step 6: Godoc Review (~2 hours)

Review all public APIs have proper documentation:
- Every exported type
- Every exported function
- Every exported constant
- Include examples where helpful

---

## Checklist

### Claude CLI Enhancements (Step 0)
- [ ] Add CLIResponse and ModelUsage types
- [ ] Switch to --output-format json
- [ ] Parse full JSON response with tokens/cost
- [ ] Add SessionID, CostUSD to CompletionResponse
- [ ] Add WithOutputFormat option
- [ ] Add WithJSONSchema option
- [ ] Add WithSessionID, WithContinue, WithResume options
- [ ] Add WithNoSessionPersistence option
- [ ] Add WithDisallowedTools, WithTools options
- [ ] Add WithDangerouslySkipPermissions option
- [ ] Add WithPermissionMode option
- [ ] Add WithSettingSources option
- [ ] Add WithAddDirs option
- [ ] Add WithSystemPrompt, WithAppendSystemPrompt options
- [ ] Add WithMaxBudgetUSD option
- [ ] Add WithFallbackModel option
- [ ] Update buildArgs() for all new options
- [ ] Tests for JSON parsing
- [ ] Tests for new options
- [ ] Integration test (skip if no claude binary)

### Documentation (Steps 1-6)
- [ ] doc.go package documentation
- [ ] README.md complete
- [ ] CONTRIBUTING.md
- [ ] CHANGELOG.md (initial version)
- [ ] examples/linear/main.go + README
- [ ] examples/conditional/main.go + README
- [ ] examples/loop/main.go + README
- [ ] examples/checkpointing/main.go + README
- [ ] examples/llm/main.go + README
- [ ] examples/observability/main.go + README
- [ ] benchmarks/graph_test.go
- [ ] benchmarks/execute_test.go
- [ ] benchmarks/checkpoint_test.go
- [ ] All godoc reviewed and improved
- [ ] All examples tested
- [ ] All quality checks pass

---

## Acceptance Criteria

### LLM Client Works with Full Features

```go
client := llm.NewClaudeCLI(
    llm.WithModel("sonnet"),
    llm.WithOutputFormat("json"),
    llm.WithDangerouslySkipPermissions(),
    llm.WithMaxBudgetUSD(1.0),
)

resp, err := client.Complete(ctx, req)
// resp.SessionID is populated
// resp.Usage.InputTokens is accurate
// resp.CostUSD reflects actual cost
```

### Examples Work

```bash
cd examples/linear && go run main.go
cd examples/conditional && go run main.go
cd examples/loop && go run main.go
cd examples/checkpointing && go run main.go
cd examples/observability && go run main.go
```

### Quality Checks Pass

```bash
go test -race ./...
go vet ./...
gofmt -s -d . | tee /dev/stderr | (! grep .)
```

---

## Reference Documents

| Document | Use For |
|----------|---------|
| `.spec/phases/PHASE-6-polish.md` | Complete templates and checklists |
| `.spec/tracking/PROGRESS.md` | Progress tracking |
| `CLAUDE.md` | Project overview and current state |
| `pkg/flowgraph/llm/*.go` | Current LLM implementation |
| `~/repos/ai-devtools/ensemble/core/runner.py` | Production Claude CLI patterns |
| `claude --help` | Full CLI flag reference |

---

## First Steps

1. **Run `claude --help`** to see all available flags
2. **Start with Step 0A** - JSON output parsing is foundational
3. **Test with actual claude binary** if available
4. **Then proceed to documentation** (Steps 1-6)

---

## Notes

- The LLM client enhancements are BLOCKING for a production-ready library
- JSON output gives us token tracking which is essential for cost management
- Session management enables multi-turn workflows
- All documentation should reference the enhanced LLM client features
- Examples should demonstrate the new options where relevant

---

## IMPORTANT: devflow Dependency

**devflow (../devflow) is BLOCKED waiting for flowgraph Phase 6.**

devflow currently has duplicate LLM code that should be removed and replaced with flowgraph imports. This is blocked until flowgraph has:

### Required for devflow Integration

| Feature | Status | Notes |
|---------|--------|-------|
| Full Claude CLI JSON parsing | Phase 6 | Token/cost extraction |
| `SessionID` in response | Phase 6 | Multi-turn tracking |
| `CostUSD` in response | Phase 6 | Budget tracking |
| `WithSessionID(id)` | Phase 6 | Session management |
| `WithContinue()` | Phase 6 | Continue last session |
| `WithResume(id)` | Phase 6 | Resume specific session |
| `WithMaxTurns(n)` | Phase 6 | Limit turns |
| `WithSystemPrompt(s)` | Phase 6 | Set system prompt |
| `WithDisallowedTools(tools)` | Phase 6 | Blacklist tools |
| `WithDangerouslySkipPermissions()` | Phase 6 | Non-interactive mode |
| `WithMaxBudgetUSD(amount)` | Phase 6 | Cap spending |

### Features devflow Will Migrate to flowgraph

Once Phase 6 LLM enhancements are done, consider if these should also live in flowgraph:

| Feature | Currently In | Notes |
|---------|-------------|-------|
| `ContextBuilder` | devflow | File context aggregation |
| `PromptLoader` | devflow | Go template prompt loading |
| `PromptBuilder` | devflow | Programmatic prompt construction |

These are generic LLM utilities, not dev-workflow specific. They probably belong in flowgraph.

### After flowgraph Phase 6

devflow will:
1. Remove duplicate ClaudeCLI from devflow/claude.go
2. Import and use flowgraph/llm.Client
3. Update all workflow nodes to use flowgraph LLM
4. Either migrate or delete ContextBuilder/PromptLoader

See `../devflow/.spec/INTEGRATION_REQUIREMENTS.md` for the full contract.
