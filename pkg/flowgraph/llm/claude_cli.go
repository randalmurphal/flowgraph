package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// OutputFormat specifies the CLI output format.
type OutputFormat string

// Output format constants.
const (
	OutputFormatText       OutputFormat = "text"
	OutputFormatJSON       OutputFormat = "json"
	OutputFormatStreamJSON OutputFormat = "stream-json"
)

// PermissionMode specifies how Claude handles tool permissions.
type PermissionMode string

// Permission mode constants.
const (
	PermissionModeDefault           PermissionMode = ""
	PermissionModeAcceptEdits       PermissionMode = "acceptEdits"
	PermissionModeBypassPermissions PermissionMode = "bypassPermissions"
)

// ClaudeCLI implements Client using the Claude CLI binary.
type ClaudeCLI struct {
	path    string
	model   string
	workdir string
	timeout time.Duration

	// Output control
	outputFormat OutputFormat
	jsonSchema   string

	// Session management
	sessionID            string
	continueSession      bool
	resumeSessionID      string
	noSessionPersistence bool

	// Tool control
	allowedTools    []string
	disallowedTools []string
	tools           []string // Exact tool set (--tools flag)

	// Permissions
	dangerouslySkipPermissions bool
	permissionMode             PermissionMode
	settingSources             []string

	// Context
	addDirs            []string
	systemPrompt       string
	appendSystemPrompt string

	// Budget and limits
	maxBudgetUSD  float64
	fallbackModel string
	maxTurns      int
}

// ClaudeOption configures ClaudeCLI.
type ClaudeOption func(*ClaudeCLI)

// NewClaudeCLI creates a new Claude CLI client.
// Assumes "claude" is available in PATH unless overridden with WithClaudePath.
// By default uses JSON output format for structured responses with token tracking.
func NewClaudeCLI(opts ...ClaudeOption) *ClaudeCLI {
	c := &ClaudeCLI{
		path:         "claude",
		timeout:      5 * time.Minute,
		outputFormat: OutputFormatJSON, // Default to JSON for rich response data
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithClaudePath sets the path to the claude binary.
func WithClaudePath(path string) ClaudeOption {
	return func(c *ClaudeCLI) { c.path = path }
}

// WithModel sets the default model.
func WithModel(model string) ClaudeOption {
	return func(c *ClaudeCLI) { c.model = model }
}

// WithWorkdir sets the working directory for claude commands.
func WithWorkdir(dir string) ClaudeOption {
	return func(c *ClaudeCLI) { c.workdir = dir }
}

// WithTimeout sets the default timeout for commands.
func WithTimeout(d time.Duration) ClaudeOption {
	return func(c *ClaudeCLI) { c.timeout = d }
}

// WithAllowedTools sets the allowed tools for claude (whitelist).
func WithAllowedTools(tools []string) ClaudeOption {
	return func(c *ClaudeCLI) { c.allowedTools = tools }
}

// WithOutputFormat sets the output format (text, json, stream-json).
// Default is json for structured responses with token tracking.
func WithOutputFormat(format OutputFormat) ClaudeOption {
	return func(c *ClaudeCLI) { c.outputFormat = format }
}

// WithJSONSchema forces structured output matching the given JSON schema.
func WithJSONSchema(schema string) ClaudeOption {
	return func(c *ClaudeCLI) { c.jsonSchema = schema }
}

// WithSessionID sets a specific session ID for conversation tracking.
func WithSessionID(id string) ClaudeOption {
	return func(c *ClaudeCLI) { c.sessionID = id }
}

// WithContinue continues the most recent session.
func WithContinue() ClaudeOption {
	return func(c *ClaudeCLI) { c.continueSession = true }
}

// WithResume resumes a specific session by ID.
func WithResume(sessionID string) ClaudeOption {
	return func(c *ClaudeCLI) { c.resumeSessionID = sessionID }
}

// WithNoSessionPersistence disables saving session data.
func WithNoSessionPersistence() ClaudeOption {
	return func(c *ClaudeCLI) { c.noSessionPersistence = true }
}

// WithDisallowedTools sets the tools to disallow (blacklist).
func WithDisallowedTools(tools []string) ClaudeOption {
	return func(c *ClaudeCLI) { c.disallowedTools = tools }
}

// WithTools specifies the exact list of available tools from the built-in set.
// Use an empty slice to disable all tools, or specify tool names like "Bash", "Edit", "Read".
// This is different from WithAllowedTools which is a whitelist filter.
func WithTools(tools []string) ClaudeOption {
	return func(c *ClaudeCLI) { c.tools = tools }
}

// WithDangerouslySkipPermissions skips all permission prompts.
// Use this for non-interactive execution in trusted environments.
// WARNING: This allows Claude to execute any tools without confirmation.
func WithDangerouslySkipPermissions() ClaudeOption {
	return func(c *ClaudeCLI) { c.dangerouslySkipPermissions = true }
}

// WithPermissionMode sets the permission handling mode.
func WithPermissionMode(mode PermissionMode) ClaudeOption {
	return func(c *ClaudeCLI) { c.permissionMode = mode }
}

// WithSettingSources specifies which setting sources to use.
// Valid values: "project", "local", "user"
func WithSettingSources(sources []string) ClaudeOption {
	return func(c *ClaudeCLI) { c.settingSources = sources }
}

// WithAddDirs adds directories to Claude's file access scope.
func WithAddDirs(dirs []string) ClaudeOption {
	return func(c *ClaudeCLI) { c.addDirs = dirs }
}

// WithSystemPrompt sets a custom system prompt, replacing the default.
func WithSystemPrompt(prompt string) ClaudeOption {
	return func(c *ClaudeCLI) { c.systemPrompt = prompt }
}

// WithAppendSystemPrompt appends to the system prompt without replacing it.
func WithAppendSystemPrompt(prompt string) ClaudeOption {
	return func(c *ClaudeCLI) { c.appendSystemPrompt = prompt }
}

// WithMaxBudgetUSD sets a maximum spending limit for the session.
func WithMaxBudgetUSD(amount float64) ClaudeOption {
	return func(c *ClaudeCLI) { c.maxBudgetUSD = amount }
}

// WithFallbackModel sets a fallback model to use if the primary is overloaded.
func WithFallbackModel(model string) ClaudeOption {
	return func(c *ClaudeCLI) { c.fallbackModel = model }
}

// WithMaxTurns limits the number of agentic turns in a conversation.
// A value of 0 means no limit.
func WithMaxTurns(n int) ClaudeOption {
	return func(c *ClaudeCLI) { c.maxTurns = n }
}

// CLIResponse represents the full JSON response from Claude CLI.
type CLIResponse struct {
	Type         string                   `json:"type"`
	Subtype      string                   `json:"subtype"`
	IsError      bool                     `json:"is_error"`
	Result       string                   `json:"result"`
	SessionID    string                   `json:"session_id"`
	DurationMS   int                      `json:"duration_ms"`
	DurationAPI  int                      `json:"duration_api_ms"`
	NumTurns     int                      `json:"num_turns"`
	TotalCostUSD float64                  `json:"total_cost_usd"`
	Usage        CLIUsage                 `json:"usage"`
	ModelUsage   map[string]CLIModelUsage `json:"modelUsage"`
}

// CLIUsage contains aggregate token usage from the CLI response.
type CLIUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// CLIModelUsage contains per-model token usage and cost.
type CLIModelUsage struct {
	InputTokens              int     `json:"inputTokens"`
	OutputTokens             int     `json:"outputTokens"`
	CacheReadInputTokens     int     `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int     `json:"cacheCreationInputTokens"`
	CostUSD                  float64 `json:"costUSD"`
}

// Complete implements Client.
func (c *ClaudeCLI) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()

	args := c.buildArgs(req)
	cmd := exec.CommandContext(ctx, c.path, args...)

	if c.workdir != "" {
		cmd.Dir = c.workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check for context cancellation first
		if ctx.Err() != nil {
			return nil, NewError("complete", ctx.Err(), false)
		}

		errMsg := sanitizeStderr(stderr.String())
		retryable := isRetryableError(errMsg)
		return nil, NewError("complete", fmt.Errorf("%w: %s", err, errMsg), retryable)
	}

	resp := c.parseResponse(stdout.Bytes())
	resp.Duration = time.Since(start)

	return resp, nil
}

// Stream implements Client.
func (c *ClaudeCLI) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	// Force stream-json output for streaming
	args := c.buildArgsWithFormat(req, OutputFormatStreamJSON)
	cmd := exec.CommandContext(ctx, c.path, args...)

	if c.workdir != "" {
		cmd.Dir = c.workdir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, NewError("stream", fmt.Errorf("create stdout pipe: %w", err), false)
	}

	if err := cmd.Start(); err != nil {
		return nil, NewError("stream", fmt.Errorf("start command: %w", err), false)
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		defer func() {
			// Wait for command to finish
			_ = cmd.Wait()
		}()

		scanner := bufio.NewScanner(stdout)
		var accumulatedContent strings.Builder

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			// Try to parse as JSON streaming event
			var event streamEvent
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				// Not JSON, treat as raw text
				accumulatedContent.WriteString(line)
				accumulatedContent.WriteString("\n")
				select {
				case ch <- StreamChunk{Content: line + "\n"}:
				case <-ctx.Done():
					ch <- StreamChunk{Error: ctx.Err()}
					return
				}
				continue
			}

			// Handle different event types
			switch event.Type {
			case "content_block_delta":
				if event.Delta != nil && event.Delta.Text != "" {
					accumulatedContent.WriteString(event.Delta.Text)
					select {
					case ch <- StreamChunk{Content: event.Delta.Text}:
					case <-ctx.Done():
						ch <- StreamChunk{Error: ctx.Err()}
						return
					}
				}
			case "message_stop":
				select {
				case ch <- StreamChunk{
					Done: true,
					Usage: &TokenUsage{
						InputTokens:  event.Usage.InputTokens,
						OutputTokens: event.Usage.OutputTokens,
						TotalTokens:  event.Usage.InputTokens + event.Usage.OutputTokens,
					},
				}:
				case <-ctx.Done():
					ch <- StreamChunk{Error: ctx.Err()}
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Error: NewError("stream", fmt.Errorf("read output: %w", err), false)}
			return
		}

		// If we didn't get a message_stop event, send final chunk
		select {
		case ch <- StreamChunk{Done: true}:
		default:
		}
	}()

	return ch, nil
}

// buildArgs constructs CLI arguments from a request using the client's configured format.
func (c *ClaudeCLI) buildArgs(req CompletionRequest) []string {
	return c.buildArgsWithFormat(req, c.outputFormat)
}

// buildArgsWithFormat constructs CLI arguments with a specific output format.
func (c *ClaudeCLI) buildArgsWithFormat(req CompletionRequest, format OutputFormat) []string {
	var args []string

	// Always use --print for non-interactive mode
	args = append(args, "--print")

	// Output format and schema
	args = c.appendOutputArgs(args, format)

	// Session management
	args = c.appendSessionArgs(args)

	// Model and prompt configuration
	args = c.appendModelArgs(args, req)

	// Tool control
	args = c.appendToolArgs(args)

	// Permissions and settings
	args = c.appendPermissionArgs(args)

	// Build and append the actual prompt from messages
	args = c.appendMessagePrompt(args, req.Messages)

	return args
}

// appendOutputArgs adds output format arguments.
func (c *ClaudeCLI) appendOutputArgs(args []string, format OutputFormat) []string {
	if format != "" && format != OutputFormatText {
		args = append(args, "--output-format", string(format))
	}
	if c.jsonSchema != "" {
		args = append(args, "--json-schema", c.jsonSchema)
	}
	return args
}

// appendSessionArgs adds session management arguments.
func (c *ClaudeCLI) appendSessionArgs(args []string) []string {
	if c.sessionID != "" {
		args = append(args, "--session-id", c.sessionID)
	}
	if c.continueSession {
		args = append(args, "--continue")
	}
	if c.resumeSessionID != "" {
		args = append(args, "--resume", c.resumeSessionID)
	}
	if c.noSessionPersistence {
		args = append(args, "--no-session-persistence")
	}
	return args
}

// appendModelArgs adds model, prompt, and limit arguments.
func (c *ClaudeCLI) appendModelArgs(args []string, req CompletionRequest) []string {
	// System prompt handling
	if c.systemPrompt != "" {
		args = append(args, "--system-prompt", c.systemPrompt)
	} else if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}
	if c.appendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", c.appendSystemPrompt)
	}

	// Model priority: request > client default
	model := c.model
	if req.Model != "" {
		model = req.Model
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	// Fallback model
	if c.fallbackModel != "" {
		args = append(args, "--fallback-model", c.fallbackModel)
	}

	// Max tokens
	if req.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", req.MaxTokens))
	}

	// Budget limit
	if c.maxBudgetUSD > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.6f", c.maxBudgetUSD))
	}

	// Max turns
	if c.maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", c.maxTurns))
	}

	return args
}

// appendToolArgs adds tool control arguments.
func (c *ClaudeCLI) appendToolArgs(args []string) []string {
	// Allowed tools (whitelist)
	for _, tool := range c.allowedTools {
		args = append(args, "--allowedTools", tool)
	}

	// Disallowed tools (blacklist)
	for _, tool := range c.disallowedTools {
		args = append(args, "--disallowed-tools", tool)
	}

	// Exact tool set
	if len(c.tools) > 0 {
		args = append(args, "--tools", strings.Join(c.tools, ","))
	}

	return args
}

// appendPermissionArgs adds permission and settings arguments.
func (c *ClaudeCLI) appendPermissionArgs(args []string) []string {
	if c.dangerouslySkipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}
	if c.permissionMode != "" {
		args = append(args, "--permission-mode", string(c.permissionMode))
	}

	// Setting sources
	if len(c.settingSources) > 0 {
		args = append(args, "--setting-sources", strings.Join(c.settingSources, ","))
	}

	// Additional directories
	for _, dir := range c.addDirs {
		args = append(args, "--add-dir", dir)
	}

	return args
}

// appendMessagePrompt converts messages to a CLI prompt and appends it.
func (c *ClaudeCLI) appendMessagePrompt(args []string, messages []Message) []string {
	// Claude CLI expects a single prompt, so concatenate user messages
	var prompt strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case RoleUser:
			prompt.WriteString(msg.Content)
			prompt.WriteString("\n")
		case RoleAssistant:
			// For conversation history, format as context
			if prompt.Len() > 0 {
				prompt.WriteString("\nAssistant: ")
				prompt.WriteString(msg.Content)
				prompt.WriteString("\n\nUser: ")
			}
		}
	}

	// Use -p flag for prompt
	promptStr := strings.TrimSpace(prompt.String())
	if promptStr != "" {
		args = append(args, "-p", promptStr)
	}

	return args
}

// parseResponse extracts response data from CLI output.
// Handles both JSON format (rich data) and text format (basic).
func (c *ClaudeCLI) parseResponse(data []byte) *CompletionResponse {
	content := strings.TrimSpace(string(data))

	// Try to parse as JSON response
	var cliResp CLIResponse
	if err := json.Unmarshal(data, &cliResp); err == nil && cliResp.Type != "" {
		return c.parseJSONResponse(&cliResp)
	}

	// Fall back to raw text response
	return &CompletionResponse{
		Content:      content,
		FinishReason: "stop",
		Model:        c.model,
		Usage: TokenUsage{
			// Token counts not available from basic CLI output
			InputTokens:  0,
			OutputTokens: 0,
			TotalTokens:  0,
		},
	}
}

// parseJSONResponse extracts rich data from a JSON CLI response.
func (c *ClaudeCLI) parseJSONResponse(cliResp *CLIResponse) *CompletionResponse {
	resp := &CompletionResponse{
		Content:   cliResp.Result,
		SessionID: cliResp.SessionID,
		CostUSD:   cliResp.TotalCostUSD,
		NumTurns:  cliResp.NumTurns,
		Usage: TokenUsage{
			InputTokens:              cliResp.Usage.InputTokens,
			OutputTokens:             cliResp.Usage.OutputTokens,
			TotalTokens:              cliResp.Usage.InputTokens + cliResp.Usage.OutputTokens,
			CacheCreationInputTokens: cliResp.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     cliResp.Usage.CacheReadInputTokens,
		},
	}

	// Determine finish reason from response type
	if cliResp.IsError {
		resp.FinishReason = "error"
	} else {
		resp.FinishReason = "stop"
	}

	// Get model from modelUsage if available
	for model := range cliResp.ModelUsage {
		resp.Model = model
		break // Use first model found
	}

	// Fall back to client's configured model
	if resp.Model == "" {
		resp.Model = c.model
	}

	return resp
}

// isRetryableError checks if an error message indicates a transient error.
func isRetryableError(errMsg string) bool {
	errLower := strings.ToLower(errMsg)
	return strings.Contains(errLower, "rate limit") ||
		strings.Contains(errLower, "timeout") ||
		strings.Contains(errLower, "overloaded") ||
		strings.Contains(errLower, "503") ||
		strings.Contains(errLower, "529")
}

// maxStderrLength limits stderr output in error messages to prevent
// leaking sensitive information and keeping errors readable.
const maxStderrLength = 500

// sanitizeStderr prepares stderr output for inclusion in error messages.
// It truncates long output and redacts common sensitive patterns.
func sanitizeStderr(stderr string) string {
	// Truncate if too long
	if len(stderr) > maxStderrLength {
		stderr = stderr[:maxStderrLength] + "... (truncated)"
	}
	return strings.TrimSpace(stderr)
}

// streamEvent represents a streaming API event from claude.
type streamEvent struct {
	Type  string       `json:"type"`
	Delta *streamDelta `json:"delta,omitempty"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

type streamDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
