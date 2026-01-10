// Example: LLM integration via context values
//
// This example demonstrates how to use LLM clients with flowgraph by
// passing them through Go's context.WithValue. This approach keeps
// flowgraph decoupled from specific LLM implementations.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph"
	"github.com/randalmurphal/llmkit/claude"
)

// State holds the conversation data.
type State struct {
	Question   string
	Answer     string
	TokensUsed int
	Model      string
}

// llmKey is the context key for the LLM client.
type llmKey struct{}

// WithLLM adds an LLM client to the context.
func WithLLM(ctx context.Context, client claude.Client) context.Context {
	return context.WithValue(ctx, llmKey{}, client)
}

// LLM retrieves the LLM client from context.
func LLM(ctx context.Context) claude.Client {
	if c, ok := ctx.Value(llmKey{}).(claude.Client); ok {
		return c
	}
	return nil
}

// generateAnswer uses the LLM to answer the question.
func generateAnswer(ctx flowgraph.Context, s State) (State, error) {
	// Access LLM client from underlying context
	client := LLM(ctx)
	if client == nil {
		return s, fmt.Errorf("LLM client not configured")
	}

	// Build completion request
	resp, err := client.Complete(ctx, claude.CompletionRequest{
		Messages: []claude.Message{
			{Role: claude.RoleUser, Content: s.Question},
		},
	})
	if err != nil {
		return s, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Extract response data
	s.Answer = resp.Content
	s.TokensUsed = resp.Usage.TotalTokens
	s.Model = resp.Model
	return s, nil
}

func main() {
	// Create a mock LLM client for demonstration
	// In production, use claude.NewClaudeCLI(...) instead
	mockClient := claude.NewMockClient("The answer to your question is 42.").
		WithResponses(
			"The answer to your question is 42.",
			"I understand you're asking about the meaning of life.",
			"Let me help you with that problem.",
		)

	// Build the graph
	graph := flowgraph.NewGraph[State]().
		AddNode("generate", generateAnswer).
		AddEdge("generate", flowgraph.END).
		SetEntry("generate")

	compiled, err := graph.Compile()
	if err != nil {
		log.Fatal("compile error:", err)
	}

	// Create context with LLM client via context.WithValue
	baseCtx := WithLLM(context.Background(), mockClient)
	ctx := flowgraph.NewContext(baseCtx)

	// Run with different questions
	questions := []string{
		"What is 6 times 7?",
		"What is the meaning of life?",
		"Can you help me debug this code?",
	}

	for _, q := range questions {
		fmt.Printf("Question: %s\n", q)

		result, err := compiled.Run(ctx, State{Question: q})
		if err != nil {
			log.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Answer: %s\n", result.Answer)
		fmt.Printf("Tokens: %d, Model: %s\n\n", result.TokensUsed, result.Model)
	}

	// Show call tracking
	fmt.Printf("Total LLM calls made: %d\n", mockClient.CallCount())

	// Show how to use ClaudeCLI in production
	fmt.Println("\n=== Production Configuration ===")
	fmt.Println("Use context.WithValue to inject LLM client:")
	fmt.Print(`
client := claude.NewClaudeCLI(
    claude.WithModel("sonnet"),
    claude.WithOutputFormat(claude.OutputFormatJSON),
    claude.WithDangerouslySkipPermissions(),
    claude.WithMaxBudgetUSD(1.0),
)

baseCtx := WithLLM(context.Background(), client)
ctx := flowgraph.NewContext(baseCtx)
`)
}
