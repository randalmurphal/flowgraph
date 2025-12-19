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

// ClaudeCLI implements Client using the Claude CLI binary.
type ClaudeCLI struct {
	path         string
	model        string
	workdir      string
	timeout      time.Duration
	allowedTools []string
}

// ClaudeOption configures ClaudeCLI.
type ClaudeOption func(*ClaudeCLI)

// NewClaudeCLI creates a new Claude CLI client.
// Assumes "claude" is available in PATH unless overridden with WithClaudePath.
func NewClaudeCLI(opts ...ClaudeOption) *ClaudeCLI {
	c := &ClaudeCLI{
		path:    "claude",
		timeout: 5 * time.Minute,
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

// WithAllowedTools sets the allowed tools for claude.
func WithAllowedTools(tools []string) ClaudeOption {
	return func(c *ClaudeCLI) { c.allowedTools = tools }
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

		errMsg := stderr.String()
		retryable := isRetryableError(errMsg)
		return nil, NewError("complete", fmt.Errorf("%w: %s", err, errMsg), retryable)
	}

	resp := c.parseResponse(stdout.Bytes())
	resp.Duration = time.Since(start)

	return resp, nil
}

// Stream implements Client.
func (c *ClaudeCLI) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	args := append(c.buildArgs(req), "--output-format", "stream-json")
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

// buildArgs constructs CLI arguments from a request.
func (c *ClaudeCLI) buildArgs(req CompletionRequest) []string {
	args := []string{"--print"}

	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	// Model priority: request > client default
	model := c.model
	if req.Model != "" {
		model = req.Model
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	if req.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", req.MaxTokens))
	}

	// Allowed tools
	for _, tool := range c.allowedTools {
		args = append(args, "--allowedTools", tool)
	}

	// Build prompt from messages
	// Claude CLI expects a single prompt, so concatenate user messages
	var prompt strings.Builder
	for _, msg := range req.Messages {
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
func (c *ClaudeCLI) parseResponse(data []byte) *CompletionResponse {
	content := strings.TrimSpace(string(data))

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

// isRetryableError checks if an error message indicates a transient error.
func isRetryableError(errMsg string) bool {
	errLower := strings.ToLower(errMsg)
	return strings.Contains(errLower, "rate limit") ||
		strings.Contains(errLower, "timeout") ||
		strings.Contains(errLower, "overloaded") ||
		strings.Contains(errLower, "503") ||
		strings.Contains(errLower, "529")
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
