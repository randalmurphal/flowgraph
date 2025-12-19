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

## Your Mission

Conduct a thorough review of the entire codebase to find:

1. **API Misalignments** - Code that doesn't match documented interfaces
2. **Claude CLI Issues** - Options or parsing that don't match actual CLI behavior
3. **Example Bugs** - Examples that won't compile or run correctly
4. **Documentation Drift** - Docs that don't match implementation
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

### Round 4: Documentation Alignment

For each doc file:

| File | Validate |
|------|----------|
| `CLAUDE.md` | Matches current structure and APIs |
| `README.md` | Install, usage, examples all work |
| `CONTRIBUTING.md` | Build/test commands work |
| `pkg/*/CLAUDE.md` | Package docs match implementation |
| `examples/*/README.md` | Match the actual example code |

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

### Priority 4: Documentation
- `CLAUDE.md` - Root guide
- `README.md` - User-facing
- `pkg/*/CLAUDE.md` - Package guides
