package model

import (
	"context"
)

// selectorKey is the context key for the model selector.
type selectorKey struct{}

// TierFunc maps a task to its tier.
type TierFunc func(task any) Tier

// Selector provides task-based model selection with override support.
// Uses any for task type to allow flexibility - higher-level packages
// can define their own task types.
type Selector struct {
	defaults    map[any]ModelName
	overrides   map[any]ModelName
	globalOver  ModelName
	tierFunc    TierFunc

	// Model names configured by user
	defaultModel  ModelName
	thinkingModel ModelName
	fastModel     ModelName
}

// SelectorOption configures a Selector.
type SelectorOption func(*Selector)

// NewSelector creates a new model selector with the given options.
func NewSelector(opts ...SelectorOption) *Selector {
	s := &Selector{
		defaults:      make(map[any]ModelName),
		overrides:     make(map[any]ModelName),
		defaultModel:  ModelSonnet,
		thinkingModel: ModelOpus,
		fastModel:     ModelHaiku,
		tierFunc:      func(_ any) Tier { return TierDefault },
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithDefaultModel sets the default model for most tasks.
func WithDefaultModel(model ModelName) SelectorOption {
	return func(s *Selector) {
		s.defaultModel = model
	}
}

// WithThinkingModel sets the model for complex reasoning tasks.
func WithThinkingModel(model ModelName) SelectorOption {
	return func(s *Selector) {
		s.thinkingModel = model
	}
}

// WithFastModel sets the model for simple, high-volume tasks.
func WithFastModel(model ModelName) SelectorOption {
	return func(s *Selector) {
		s.fastModel = model
	}
}

// WithTaskOverride sets a model override for a specific task.
func WithTaskOverride(task any, model ModelName) SelectorOption {
	return func(s *Selector) {
		s.overrides[task] = model
	}
}

// WithTaskOverrides sets multiple task overrides at once.
func WithTaskOverrides(overrides map[any]ModelName) SelectorOption {
	return func(s *Selector) {
		for task, model := range overrides {
			s.overrides[task] = model
		}
	}
}

// WithDefaults sets the default task-to-model mapping.
func WithDefaults(defaults map[any]ModelName) SelectorOption {
	return func(s *Selector) {
		s.defaults = defaults
	}
}

// WithGlobalOverride sets a global model that overrides all selections.
func WithGlobalOverride(model ModelName) SelectorOption {
	return func(s *Selector) {
		s.globalOver = model
	}
}

// WithTierFunc sets the function to determine a task's tier.
func WithTierFunc(fn TierFunc) SelectorOption {
	return func(s *Selector) {
		s.tierFunc = fn
	}
}

// Select returns the appropriate model for the given task.
// Priority order: global override > task override > task default > tier model > sonnet fallback
func (s *Selector) Select(task any) ModelName {
	// Global override wins
	if s.globalOver != "" {
		return s.globalOver
	}

	// Task-specific override
	if model, ok := s.overrides[task]; ok {
		return model
	}

	// Task default from defaults map
	if model, ok := s.defaults[task]; ok {
		return model
	}

	// Look up the tier for this task
	tier := s.tierFunc(task)
	return s.SelectForTier(tier)
}

// SelectForTier returns the model for a specific tier.
func (s *Selector) SelectForTier(tier Tier) ModelName {
	// Global override wins
	if s.globalOver != "" {
		return s.globalOver
	}

	switch tier {
	case TierThinking:
		return s.thinkingModel
	case TierFast:
		return s.fastModel
	default:
		return s.defaultModel
	}
}

// Clone returns a copy of the selector with the same configuration.
func (s *Selector) Clone() *Selector {
	overrides := make(map[any]ModelName, len(s.overrides))
	for k, v := range s.overrides {
		overrides[k] = v
	}

	defaults := make(map[any]ModelName, len(s.defaults))
	for k, v := range s.defaults {
		defaults[k] = v
	}

	return &Selector{
		defaults:      defaults,
		overrides:     overrides,
		globalOver:    s.globalOver,
		defaultModel:  s.defaultModel,
		thinkingModel: s.thinkingModel,
		fastModel:     s.fastModel,
		tierFunc:      s.tierFunc,
	}
}

// WithGlobal returns a new selector with a global override applied.
func (s *Selector) WithGlobal(model ModelName) *Selector {
	clone := s.Clone()
	clone.globalOver = model
	return clone
}

// NewContext returns a new context with the selector attached.
func NewContext(ctx context.Context, selector *Selector) context.Context {
	return context.WithValue(ctx, selectorKey{}, selector)
}

// FromContext retrieves the selector from the context.
// Returns a default selector if none is present.
func FromContext(ctx context.Context) *Selector {
	if s, ok := ctx.Value(selectorKey{}).(*Selector); ok {
		return s
	}
	return NewSelector()
}
