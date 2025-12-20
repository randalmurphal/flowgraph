// Package model provides model selection, escalation chains, and cost tracking.
//
// The package implements a tiered model selection strategy:
//   - Thinking tier (opus): Complex reasoning, architecture, risk assessment
//   - Default tier (sonnet): Implementation, review, general tasks
//   - Fast tier (haiku): Search, simple transforms, high-volume
//
// This package is designed to be task-type agnostic. Higher-level packages
// (like devflow) can define their own task types and map them to model tiers.
package model

// ModelName represents a Claude model name.
type ModelName string

const (
	ModelOpus   ModelName = "opus"
	ModelSonnet ModelName = "sonnet"
	ModelHaiku  ModelName = "haiku"
)

// Tier represents a model capability tier.
type Tier int

const (
	TierFast Tier = iota
	TierDefault
	TierThinking
)

// String returns the tier name.
func (t Tier) String() string {
	switch t {
	case TierFast:
		return "fast"
	case TierDefault:
		return "default"
	case TierThinking:
		return "thinking"
	default:
		return "unknown"
	}
}

// TierForModel returns the tier for a given model.
func TierForModel(model ModelName) Tier {
	switch model {
	case ModelOpus:
		return TierThinking
	case ModelHaiku:
		return TierFast
	default:
		return TierDefault
	}
}
