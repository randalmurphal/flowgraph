package llm

import (
	"encoding/json"
	"time"
)

// CompletionRequest configures an LLM completion call.
type CompletionRequest struct {
	// Prompt configuration
	SystemPrompt string    `json:"system_prompt,omitempty"`
	Messages     []Message `json:"messages"`

	// Model configuration
	Model       string  `json:"model,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`

	// Tool use
	Tools []Tool `json:"tools,omitempty"`

	// Provider-specific options
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
}

// Add calculates total tokens and adds to existing usage.
func (u *TokenUsage) Add(other TokenUsage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.TotalTokens += other.TotalTokens
}

// StreamChunk is a piece of a streaming response.
type StreamChunk struct {
	Content   string      `json:"content,omitempty"`
	ToolCalls []ToolCall  `json:"tool_calls,omitempty"`
	Usage     *TokenUsage `json:"usage,omitempty"` // Only set in final chunk
	Done      bool        `json:"done"`
	Error     error       `json:"-"` // Non-nil if streaming failed
}
