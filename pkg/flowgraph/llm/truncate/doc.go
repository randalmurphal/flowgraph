// Package truncate provides text truncation utilities for managing LLM context.
//
// When building prompts for language models, text often needs to be truncated
// to fit within token limits. This package provides flexible truncation with
// multiple strategies.
//
// # Strategies
//
// Three truncation strategies are available:
//
//   - FromEnd: Remove content from the end (default)
//   - FromMiddle: Remove content from the middle, keeping start and end
//   - FromStart: Remove content from the start
//
// # Basic Usage
//
// Create a truncator and truncate text:
//
//	tr := truncate.NewFromEnd()
//	result, truncated := tr.Truncate("very long text...", 100)
//
// Or use a specific strategy:
//
//	tr := truncate.New(truncate.FromMiddle)
//	result, truncated := tr.Truncate(text, maxTokens)
//
// # Custom Token Counter
//
// By default, truncation uses an estimating counter (~4 chars/token).
// For more accurate results, provide a custom counter:
//
//	tr := truncate.NewFromEnd().WithCounter(myCounter)
//
// # Convenience Functions
//
// For simple one-off truncation:
//
//	result := truncate.ToTokens(text, 100)   // Truncate to 100 tokens
//	result := truncate.ToLines(text, 50)     // Truncate to 50 lines
//	result := truncate.ToLength(text, 500)   // Truncate to 500 characters
//	result := truncate.Smart(text, 500)      // Truncate at word boundaries
//
// # UTF-8 Support
//
// All truncation properly handles UTF-8 text, counting runes rather than
// bytes to ensure multi-byte characters are not split.
package truncate
