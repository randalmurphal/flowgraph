# flowgraph Review Session

**Purpose**: Thorough multi-round review to find and fix any issues before v0.1.0 release.

---

## Status

All implementation phases complete. Release branch `release/v0.1.0` created.

**Coverage**: 88.2% overall
- flowgraph: 89.1%
- checkpoint: 91.3%
- llm: 83.8%
- observability: 90.6%

---

## CRITICAL: docs/ Directory is Severely Out of Date

The `docs/` directory contains **pre-implementation planning specs** that reference non-existent APIs. These must be **rewritten from scratch** to match actual implementation.

| Non-Existent API | Actual Implementation | Affected Files |
|------------------|----------------------|----------------|
| `PostgresStore` | Only `MemoryStore`, `SQLiteStore` exist | OVERVIEW, ARCHITECTURE, API_REFERENCE, TESTING_STRATEGY |
| `TemporalStore` | Not implemented | OVERVIEW |
| `ClaudeCLIClient` | `ClaudeCLI` | OVERVIEW, ARCHITECTURE, API_REFERENCE |
| `NewClaudeCLIClient()` | `NewClaudeCLI()` | API_REFERENCE |
| `RunWithCheckpointing()` | `Run()` with `WithCheckpointing(store)` option | ALL |
| `RunWithOptions()` | Just `Run()` with variadic options | API_REFERENCE |
| `CheckpointStore.Save(runID, nodeID, state)` | `Save(ctx, *Checkpoint)` | OVERVIEW, ARCHITECTURE, API_REFERENCE |
| `CheckpointStore.Load(runID, nodeID)` | `Load(ctx, runID)` | OVERVIEW, ARCHITECTURE, API_REFERENCE |
| "Metrics (Future)" | **Implemented** in `observability/metrics.go` | ARCHITECTURE |
| "Tracing (Future)" | **Implemented** in `observability/tracing.go` | ARCHITECTURE |
| `WithClaudeBinary()` | `WithClaudePath()` | API_REFERENCE |
| `MockResponse` struct | Use `WithResponses()`, `WithHandler()` | API_REFERENCE |
| `Context.Checkpoint()` method | Checkpointing via `Run()` options | OVERVIEW, ARCHITECTURE |

**Decision**: Either rewrite these docs to match implementation, OR delete and consolidate into `CLAUDE.md` hierarchy.

---

## Your Mission

Conduct a thorough review of the entire codebase to find:

1. **API Misalignments** - Code that doesn't match documented interfaces
2. **Claude CLI Issues** - Options or parsing that don't match actual CLI behavior
3. **Example Bugs** - Examples that won't compile or run correctly
4. **Documentation Drift** - Docs that don't match implementation (MAJOR - see above)
5. **Test Gaps** - Missing test coverage for edge cases
6. **Inconsistencies** - Naming, patterns, or style that varies

---

## Review Strategy

### Round 1: Claude CLI Validation

The most critical component is `pkg/flowgraph/llm/claude_cli.go`. It MUST match actual Claude Code CLI behavior.

**Reference**: `~/repos/ai-devtools/ensemble/core/runner.py` contains battle-tested Python patterns.

Validate:

| Aspect | Check |
|--------|-------|
| CLI flags | Do `buildArgs()` flags match `claude --help`? |
| JSON parsing | Does `parseResponse()` match actual CLI JSON output? |
| Token tracking | Are `modelUsage` fields correctly named (camelCase vs snake_case)? |
| Error handling | Are error types and retry logic sound? |
| Options | Do all `WithXxx` options map to valid CLI flags? |

Run actual validation:
```bash
# Get actual CLI help
claude --help

# Test JSON output format
echo "hello" | claude -p --output-format json "respond with ok"
```

### Round 2: Examples Validation

Every example must compile and demonstrate correct usage.

```bash
cd examples/linear && go build && go run main.go
cd examples/conditional && go build && go run main.go
cd examples/loop && go build && go run main.go
cd examples/checkpointing && go build && go run main.go
cd examples/llm && go build && go run main.go
cd examples/observability && go build && go run main.go
```

Check:
- Do they compile without errors?
- Do they use APIs correctly (not deprecated patterns)?
- Are imports correct?
- Do comments match what the code does?

### Round 3: API Surface Review

For each public API in `pkg/flowgraph/`:

1. Read the godoc comment
2. Read the implementation
3. Verify they match
4. Check all callers use it correctly

Key APIs to validate:
- `NewGraph[S]()` and builder methods
- `Compile()` error conditions
- `Run()` options and behavior
- `Resume()` and `ResumeFrom()`
- Context injection (`WithLLM`, `WithCheckpointing`, etc.)

### Round 4: Documentation Overhaul

**The `docs/` directory needs major work:**

1. **Option A - Rewrite**: Update each file to match actual APIs
2. **Option B - Delete and consolidate**: Move essential content to `CLAUDE.md` files, delete the rest

Recommended: **Option B** - The `CLAUDE.md` hierarchy is cleaner and easier to maintain.

**Files to update/verify:**

| File | Action |
|------|--------|
| `CLAUDE.md` | Already updated - verify accuracy |
| `README.md` | Already updated - verify accuracy |
| `CONTRIBUTING.md` | Already updated - verify accuracy |
| `pkg/*/CLAUDE.md` | Already created - verify accuracy |
| `examples/*/README.md` | Verify they match code |
| `docs/OVERVIEW.md` | **DELETE or REWRITE** - wrong APIs |
| `docs/ARCHITECTURE.md` | **DELETE or REWRITE** - wrong APIs |
| `docs/API_REFERENCE.md` | **DELETE or REWRITE** - wrong APIs |
| `docs/TESTING_STRATEGY.md` | **REWRITE** - wrong store interface |
| `docs/IMPLEMENTATION_GUIDE.md` | Keep (historical) or delete |
| `docs/GO_PATTERNS.md` | Keep (general Go guidance) |
| `docs/DECISIONS.md` | Already created - keep |

### Round 5: Test Coverage Analysis

```bash
go test -coverprofile=coverage.out ./pkg/flowgraph/...
go tool cover -func=coverage.out | grep -v "100.0%"
```

For each function under 80%:
- Is it tested via integration tests?
- Is it dead code?
- Does it need more tests?

---

## Known Reference Patterns

From `~/repos/ai-devtools/ensemble/core/runner.py`:

### CLI Command Building
```python
cmd = [
    self.claude_path,
    '-p',
    '--output-format', 'json',
    '--dangerously-skip-permissions',
    '--setting-sources', 'project,local',
]
```

### Token Usage Parsing
```python
model_usage = output.get('modelUsage', {})
for model_id, data in model_usage.items():
    usage.models[model_id] = ModelTokenUsage(
        model_id=model_id,
        input_tokens=data.get('inputTokens', 0),
        output_tokens=data.get('outputTokens', 0),
        cache_read_tokens=data.get('cacheReadInputTokens', 0),
        cache_creation_tokens=data.get('cacheCreationInputTokens', 0),
        cost_usd=data.get('costUSD', 0.0),
    )
```

Note the field names: `inputTokens` (camelCase), `cacheReadInputTokens`, `costUSD`.

---

## Quality Checklist

After review, all must be true:

- [ ] All examples compile and run
- [ ] All tests pass with `-race`
- [ ] `go vet ./...` clean
- [ ] `gofmt -s -d .` shows no changes
- [ ] Claude CLI options match `claude --help`
- [ ] JSON parsing matches actual CLI output
- [ ] Documentation matches implementation
- [ ] No dead code or unused exports

---

## Deliverables

1. **Fix any issues found** - Direct fixes, not just reports
2. **Update docs if needed** - Keep everything aligned
3. **Commit fixes** - Clean commits with clear messages
4. **Update this file** - Mark review complete when done

---

## After Review

Once all checks pass:

1. Remove `.spec/tracking/` (no longer needed)
2. Archive `.spec/phases/` (historical reference)
3. Ensure `.spec/decisions/` is preserved (ADRs are permanent)
4. Update CLAUDE.md to remove "v0.1.0 Release Ready" status → just "v0.1.0"
5. Tag `v0.1.0` on the release branch

---

## Files to Review

### Priority 0: Documentation Cleanup (BLOCKING)
The `docs/` directory has incorrect API references. Clean this up first:
- `docs/OVERVIEW.md` - Delete or rewrite completely
- `docs/ARCHITECTURE.md` - Delete or rewrite completely
- `docs/API_REFERENCE.md` - Delete or rewrite completely
- `docs/TESTING_STRATEGY.md` - Update store interface references
- `docs/IMPLEMENTATION_GUIDE.md` - Keep as historical or delete
- `docs/GO_PATTERNS.md` - Keep (general Go patterns)
- `docs/DECISIONS.md` - Keep (already accurate)

### Priority 1: LLM Integration
- `pkg/flowgraph/llm/claude_cli.go` - CLI integration
- `pkg/flowgraph/llm/request.go` - Types match CLI output
- `pkg/flowgraph/llm/internal_test.go` - JSON parsing tests

### Priority 2: Examples
- `examples/*/main.go` - All must compile and run
- `examples/*/README.md` - Must match code

### Priority 3: Core
- `pkg/flowgraph/*.go` - Public API consistency
- `pkg/flowgraph/*_test.go` - Coverage gaps

### Priority 4: Documentation Verification
- `CLAUDE.md` - Root guide (already updated)
- `README.md` - User-facing (already updated)
- `pkg/*/CLAUDE.md` - Package guides (already created)
- `CONTRIBUTING.md` - Development guide (already created)
