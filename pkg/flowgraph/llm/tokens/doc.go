// Package tokens provides token counting and budget management for LLM prompts.
//
// Token estimation is based on the rule-of-thumb that approximately 4 characters
// equals 1 token for English text. This provides a fast estimation without
// requiring a model-specific tokenizer.
//
// # Counter
//
// The Counter interface provides token counting methods:
//
//	counter := tokens.NewEstimatingCounter()
//	count := counter.Count("Hello, world!")     // ~3 tokens
//	fits := counter.FitsInLimit("text", 1000)   // true if <= 1000 tokens
//
// For one-off counting, use the convenience function:
//
//	count := tokens.EstimateTokens("Hello, world!")
//
// # Budget
//
// Budget helps allocate tokens across prompt components:
//
//	budget := tokens.NewBudget(100000)
//	// Default allocation: 20% system, 40% context, 30% user, 10% reserved
//	budget.FitsSystem(text)                     // check system prompt
//	budget.FitsContext(text)                    // check context
//	budget.RemainingContext(usedTokens)         // remaining context budget
//
// Custom allocations:
//
//	budget := tokens.NewBudgetWithAllocation(
//	    100000,  // total
//	    30,      // 30% system
//	    40,      // 40% context
//	    20,      // 20% user
//	    10,      // 10% reserved
//	)
//
// # Model Limits
//
// Get context window sizes for common models:
//
//	limit := tokens.GetModelLimit("claude-opus-4")  // 200000
//	limit := tokens.GetModelLimit("unknown")        // 100000 (default)
//
// See ModelLimits for the complete map of model context windows.
package tokens
