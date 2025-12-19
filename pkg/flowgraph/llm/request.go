package llm

import (
	"encoding/json"
	"time"
)

// CompletionRequest configures an LLM completion call.
type CompletionRequest struct {
	// SystemPrompt sets the system message that guides the model's behavior.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Messages is the conversation history to send to the model.
	Messages []Message `json:"messages"`

	// Model specifies which model to use (e.g., "claude-sonnet-4-20250514").
	Model string `json:"model,omitempty"`

	// MaxTokens limits the response length.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls response randomness (0.0 = deterministic, 1.0 = creative).
	Temperature float64 `json:"temperature,omitempty"`

	// Tools lists available tools the model can invoke.
	Tools []Tool `json:"tools,omitempty"`

	// Options holds provider-specific configuration not covered by standard fields.
	Options map[string]any `json:"options,omitempty"`
}

// Message is a conversation turn.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"` // For tool results
}

// Role identifies the message sender.
type Role string

// Standard message roles.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
	RoleSystem    Role = "system"
)

// Tool defines an available tool for the LLM.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// CompletionResponse is the output of a completion call.
type CompletionResponse struct {
	Content      string        `json:"content"`
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`
	Usage        TokenUsage    `json:"usage"`
	Model        string        `json:"model"`
	FinishReason string        `json:"finish_reason"`
	Duration     time.Duration `json:"duration"`

	// Claude CLI specific fields (populated when using JSON output)
	SessionID string  `json:"session_id,omitempty"`
	CostUSD   float64 `json:"cost_usd,omitempty"`
	NumTurns  int     `json:"num_turns,omitempty"`
}

// ToolCall represents a tool invocation request from the LLM.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`

	// Cache-related tokens (Claude specific)
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// Add calculates total tokens and adds to existing usage.
func (u *TokenUsage) Add(other TokenUsage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.TotalTokens += other.TotalTokens
	u.CacheCreationInputTokens += other.CacheCreationInputTokens
	u.CacheReadInputTokens += other.CacheReadInputTokens
}

// StreamChunk is a piece of a streaming response.
type StreamChunk struct {
	Content   string      `json:"content,omitempty"`
	ToolCalls []ToolCall  `json:"tool_calls,omitempty"`
	Usage     *TokenUsage `json:"usage,omitempty"` // Only set in final chunk
	Done      bool        `json:"done"`
	Error     error       `json:"-"` // Non-nil if streaming failed
}
