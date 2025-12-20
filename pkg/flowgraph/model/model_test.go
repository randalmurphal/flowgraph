package model

import (
	"context"
	"sync"
	"testing"
)

// TaskType is a test-local type to simulate what devflow would define.
type TaskType string

const (
	TaskInvestigate TaskType = "investigate"
	TaskImplement   TaskType = "implement"
	TaskSearch      TaskType = "search"
)

// testTierFunc returns a tier function for testing.
func testTierFunc() TierFunc {
	return func(task any) Tier {
		switch task.(TaskType) {
		case TaskInvestigate:
			return TierThinking
		case TaskSearch:
			return TierFast
		default:
			return TierDefault
		}
	}
}

func TestTierForModel(t *testing.T) {
	tests := []struct {
		model        ModelName
		expectedTier Tier
	}{
		{ModelOpus, TierThinking},
		{ModelSonnet, TierDefault},
		{ModelHaiku, TierFast},
		{ModelName("unknown"), TierDefault},
	}

	for _, tt := range tests {
		t.Run(string(tt.model), func(t *testing.T) {
			tier := TierForModel(tt.model)
			if tier != tt.expectedTier {
				t.Errorf("TierForModel(%s) = %s, want %s", tt.model, tier, tt.expectedTier)
			}
		})
	}
}

func TestTierString(t *testing.T) {
	tests := []struct {
		tier     Tier
		expected string
	}{
		{TierFast, "fast"},
		{TierDefault, "default"},
		{TierThinking, "thinking"},
		{Tier(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.tier.String(); got != tt.expected {
				t.Errorf("Tier(%d).String() = %s, want %s", tt.tier, got, tt.expected)
			}
		})
	}
}

func TestSelectorSelect(t *testing.T) {
	t.Run("default behavior with tier func", func(t *testing.T) {
		s := NewSelector(WithTierFunc(testTierFunc()))

		// Thinking tier tasks get opus
		if got := s.Select(TaskInvestigate); got != ModelOpus {
			t.Errorf("Select(TaskInvestigate) = %s, want %s", got, ModelOpus)
		}

		// Default tier tasks get sonnet
		if got := s.Select(TaskImplement); got != ModelSonnet {
			t.Errorf("Select(TaskImplement) = %s, want %s", got, ModelSonnet)
		}

		// Fast tier tasks get haiku
		if got := s.Select(TaskSearch); got != ModelHaiku {
			t.Errorf("Select(TaskSearch) = %s, want %s", got, ModelHaiku)
		}
	})

	t.Run("with custom tier models", func(t *testing.T) {
		s := NewSelector(
			WithTierFunc(testTierFunc()),
			WithThinkingModel(ModelSonnet),
			WithDefaultModel(ModelHaiku),
			WithFastModel(ModelHaiku),
		)

		if got := s.Select(TaskInvestigate); got != ModelSonnet {
			t.Errorf("Select(TaskInvestigate) = %s, want %s", got, ModelSonnet)
		}
		if got := s.Select(TaskImplement); got != ModelHaiku {
			t.Errorf("Select(TaskImplement) = %s, want %s", got, ModelHaiku)
		}
	})

	t.Run("with task override", func(t *testing.T) {
		s := NewSelector(
			WithTierFunc(testTierFunc()),
			WithTaskOverride(TaskImplement, ModelOpus),
		)

		// Overridden task
		if got := s.Select(TaskImplement); got != ModelOpus {
			t.Errorf("Select(TaskImplement) = %s, want %s", got, ModelOpus)
		}

		// Non-overridden task uses tier func
		if got := s.Select(TaskSearch); got != ModelHaiku {
			t.Errorf("Select(TaskSearch) = %s, want %s", got, ModelHaiku)
		}
	})

	t.Run("with defaults map", func(t *testing.T) {
		defaults := map[any]ModelName{
			TaskInvestigate: ModelOpus,
			TaskImplement:   ModelSonnet,
			TaskSearch:      ModelHaiku,
		}
		s := NewSelector(WithDefaults(defaults))

		if got := s.Select(TaskInvestigate); got != ModelOpus {
			t.Errorf("Select(TaskInvestigate) = %s, want %s", got, ModelOpus)
		}
	})

	t.Run("with global override", func(t *testing.T) {
		s := NewSelector(
			WithTierFunc(testTierFunc()),
			WithGlobalOverride(ModelHaiku),
		)

		// All tasks get the global override
		if got := s.Select(TaskInvestigate); got != ModelHaiku {
			t.Errorf("Select(TaskInvestigate) = %s, want %s", got, ModelHaiku)
		}
		if got := s.Select(TaskImplement); got != ModelHaiku {
			t.Errorf("Select(TaskImplement) = %s, want %s", got, ModelHaiku)
		}
	})

	t.Run("priority order", func(t *testing.T) {
		s := NewSelector(
			WithTierFunc(testTierFunc()),
			WithTaskOverride(TaskImplement, ModelHaiku),
			WithGlobalOverride(ModelOpus),
		)

		// Global override takes precedence over task override
		if got := s.Select(TaskImplement); got != ModelOpus {
			t.Errorf("Select(TaskImplement) = %s, want %s (global should win)", got, ModelOpus)
		}
	})
}

func TestSelectorSelectForTier(t *testing.T) {
	s := NewSelector(
		WithThinkingModel(ModelOpus),
		WithDefaultModel(ModelSonnet),
		WithFastModel(ModelHaiku),
	)

	tests := []struct {
		tier     Tier
		expected ModelName
	}{
		{TierThinking, ModelOpus},
		{TierDefault, ModelSonnet},
		{TierFast, ModelHaiku},
	}

	for _, tt := range tests {
		t.Run(tt.tier.String(), func(t *testing.T) {
			if got := s.SelectForTier(tt.tier); got != tt.expected {
				t.Errorf("SelectForTier(%s) = %s, want %s", tt.tier, got, tt.expected)
			}
		})
	}
}

func TestSelectorClone(t *testing.T) {
	original := NewSelector(
		WithTaskOverride(TaskImplement, ModelOpus),
		WithGlobalOverride(ModelHaiku),
	)

	clone := original.Clone()

	// Clone should have same values
	if got := clone.Select(TaskImplement); got != ModelHaiku { // global wins
		t.Errorf("Clone.Select(TaskImplement) = %s, want %s", got, ModelHaiku)
	}

	// Modifying clone shouldn't affect original
	clone.overrides[TaskSearch] = ModelOpus
	if _, ok := original.overrides[TaskSearch]; ok {
		t.Error("Modifying clone affected original")
	}
}

func TestSelectorWithGlobal(t *testing.T) {
	original := NewSelector(WithTierFunc(testTierFunc()))
	modified := original.WithGlobal(ModelOpus)

	// Original should be unchanged
	if got := original.Select(TaskSearch); got != ModelHaiku {
		t.Errorf("Original.Select(TaskSearch) = %s, want %s", got, ModelHaiku)
	}

	// Modified should use global
	if got := modified.Select(TaskSearch); got != ModelOpus {
		t.Errorf("Modified.Select(TaskSearch) = %s, want %s", got, ModelOpus)
	}
}

func TestSelectorContext(t *testing.T) {
	ctx := context.Background()
	s := NewSelector(WithGlobalOverride(ModelOpus))

	// Add to context
	ctx = NewContext(ctx, s)

	// Retrieve from context
	retrieved := FromContext(ctx)
	if got := retrieved.Select(TaskSearch); got != ModelOpus {
		t.Errorf("FromContext().Select(TaskSearch) = %s, want %s", got, ModelOpus)
	}

	// FromContext with no selector returns default
	defaultSelector := FromContext(context.Background())
	if got := defaultSelector.Select(TaskSearch); got != ModelSonnet { // default tier
		t.Errorf("FromContext(empty).Select(TaskSearch) = %s, want %s", got, ModelSonnet)
	}
}

func TestEscalationChainNext(t *testing.T) {
	tests := []struct {
		name      string
		chain     EscalationChain
		current   ModelName
		attempt   int
		wantModel ModelName
		wantOK    bool
	}{
		{
			name:      "escalate from sonnet to opus",
			chain:     DefaultEscalation,
			current:   ModelSonnet,
			attempt:   1,
			wantModel: ModelOpus,
			wantOK:    true,
		},
		{
			name:      "already at opus stays at opus",
			chain:     DefaultEscalation,
			current:   ModelOpus,
			attempt:   1,
			wantModel: ModelOpus,
			wantOK:    true,
		},
		{
			name:      "max attempts exceeded",
			chain:     DefaultEscalation,
			current:   ModelSonnet,
			attempt:   3,
			wantModel: "",
			wantOK:    false,
		},
		{
			name:      "no escalation chain retries same model",
			chain:     NoEscalation,
			current:   ModelSonnet,
			attempt:   1,
			wantModel: ModelSonnet,
			wantOK:    true,
		},
		{
			name:      "full chain escalation",
			chain:     FullEscalation,
			current:   ModelHaiku,
			attempt:   1,
			wantModel: ModelSonnet,
			wantOK:    true,
		},
		{
			name:      "model not in chain starts at beginning",
			chain:     DefaultEscalation,
			current:   ModelHaiku,
			attempt:   0,
			wantModel: ModelSonnet,
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotModel, gotOK := tt.chain.Next(tt.current, tt.attempt)
			if gotModel != tt.wantModel || gotOK != tt.wantOK {
				t.Errorf("Next(%s, %d) = (%s, %v), want (%s, %v)",
					tt.current, tt.attempt, gotModel, gotOK, tt.wantModel, tt.wantOK)
			}
		})
	}
}

func TestEscalationChainCanEscalate(t *testing.T) {
	tests := []struct {
		name    string
		chain   EscalationChain
		current ModelName
		want    bool
	}{
		{"sonnet can escalate", DefaultEscalation, ModelSonnet, true},
		{"opus cannot escalate", DefaultEscalation, ModelOpus, false},
		{"haiku in full chain can escalate", FullEscalation, ModelHaiku, true},
		{"no escalation chain", NoEscalation, ModelSonnet, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chain.CanEscalate(tt.current); got != tt.want {
				t.Errorf("CanEscalate(%s) = %v, want %v", tt.current, got, tt.want)
			}
		})
	}
}

func TestEscalationChainHighestModel(t *testing.T) {
	tests := []struct {
		name  string
		chain EscalationChain
		want  ModelName
	}{
		{"default escalation", DefaultEscalation, ModelOpus},
		{"full escalation", FullEscalation, ModelOpus},
		{"no escalation", NoEscalation, ModelSonnet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chain.HighestModel(); got != tt.want {
				t.Errorf("HighestModel() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestEscalationState(t *testing.T) {
	t.Run("progressive escalation", func(t *testing.T) {
		state := NewEscalationState(&FullEscalation, ModelHaiku)

		// First failure should escalate to sonnet
		if !state.RecordFailure(nil) {
			t.Error("Expected more attempts after first failure")
		}
		if state.CurrentModel != ModelSonnet {
			t.Errorf("After first failure: model = %s, want %s", state.CurrentModel, ModelSonnet)
		}

		// Second failure should escalate to opus
		if !state.RecordFailure(nil) {
			t.Error("Expected more attempts after second failure")
		}
		if state.CurrentModel != ModelOpus {
			t.Errorf("After second failure: model = %s, want %s", state.CurrentModel, ModelOpus)
		}
	})

	t.Run("exhaustion", func(t *testing.T) {
		chain := EscalationChain{Models: []ModelName{ModelSonnet}, MaxAttempts: 2}
		state := NewEscalationState(&chain, ModelSonnet)

		state.RecordFailure(nil)
		state.RecordFailure(nil)

		if !state.Exhausted() {
			t.Error("Expected state to be exhausted after max attempts")
		}
	})

	t.Run("nil chain uses default", func(t *testing.T) {
		state := NewEscalationState(nil, ModelSonnet)
		if state.Chain != &DefaultEscalation {
			t.Error("Expected nil chain to use DefaultEscalation")
		}
	})
}

func TestCostTracker(t *testing.T) {
	t.Run("record and retrieve", func(t *testing.T) {
		tracker := NewCostTracker()

		tracker.Record(ModelSonnet, 1000, 500)
		tracker.Record(ModelSonnet, 500, 250)
		tracker.Record(ModelOpus, 2000, 1000)

		sonnetUsage := tracker.Usage(ModelSonnet)
		if sonnetUsage.InputTokens != 1500 || sonnetUsage.OutputTokens != 750 || sonnetUsage.Requests != 2 {
			t.Errorf("Sonnet usage = %+v, want {Input:1500, Output:750, Requests:2}", sonnetUsage)
		}

		opusUsage := tracker.Usage(ModelOpus)
		if opusUsage.InputTokens != 2000 || opusUsage.OutputTokens != 1000 || opusUsage.Requests != 1 {
			t.Errorf("Opus usage = %+v, want {Input:2000, Output:1000, Requests:1}", opusUsage)
		}
	})

	t.Run("summary", func(t *testing.T) {
		tracker := NewCostTracker()
		tracker.Record(ModelSonnet, 100, 50)
		tracker.Record(ModelHaiku, 200, 100)

		summary := tracker.Summary()
		if len(summary) != 2 {
			t.Errorf("Summary has %d entries, want 2", len(summary))
		}

		// Verify it's a copy
		summary[ModelSonnet] = Usage{InputTokens: 999}
		if tracker.Usage(ModelSonnet).InputTokens == 999 {
			t.Error("Summary returned reference instead of copy")
		}
	})

	t.Run("total usage", func(t *testing.T) {
		tracker := NewCostTracker()
		tracker.Record(ModelSonnet, 100, 50)
		tracker.Record(ModelOpus, 200, 100)
		tracker.Record(ModelHaiku, 50, 25)

		total := tracker.TotalUsage()
		if total.InputTokens != 350 || total.OutputTokens != 175 || total.Requests != 3 {
			t.Errorf("TotalUsage() = %+v, want {Input:350, Output:175, Requests:3}", total)
		}
	})

	t.Run("estimated cost", func(t *testing.T) {
		tracker := NewCostTracker()
		// 1M input tokens at sonnet = $3.00
		// 1M output tokens at sonnet = $15.00
		tracker.Record(ModelSonnet, 1_000_000, 1_000_000)

		cost := tracker.EstimatedCost()
		expected := 3.0 + 15.0
		if cost != expected {
			t.Errorf("EstimatedCost() = %f, want %f", cost, expected)
		}
	})

	t.Run("cost by model", func(t *testing.T) {
		tracker := NewCostTracker()
		tracker.Record(ModelSonnet, 1_000_000, 0)
		tracker.Record(ModelOpus, 1_000_000, 0)

		costs := tracker.EstimatedCostByModel()
		if costs[ModelSonnet] != 3.0 {
			t.Errorf("Sonnet cost = %f, want 3.0", costs[ModelSonnet])
		}
		if costs[ModelOpus] != 15.0 {
			t.Errorf("Opus cost = %f, want 15.0", costs[ModelOpus])
		}
	})

	t.Run("reset", func(t *testing.T) {
		tracker := NewCostTracker()
		tracker.Record(ModelSonnet, 1000, 500)
		tracker.Reset()

		if usage := tracker.Usage(ModelSonnet); usage.InputTokens != 0 {
			t.Error("Reset did not clear usage")
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		tracker := NewCostTracker()
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				tracker.Record(ModelSonnet, 100, 50)
			}()
		}

		wg.Wait()

		usage := tracker.Usage(ModelSonnet)
		if usage.Requests != 100 {
			t.Errorf("Concurrent requests = %d, want 100", usage.Requests)
		}
	})
}

func TestUsage(t *testing.T) {
	t.Run("add", func(t *testing.T) {
		u1 := Usage{InputTokens: 100, OutputTokens: 50, Requests: 1}
		u2 := Usage{InputTokens: 200, OutputTokens: 100, Requests: 2}

		u1.Add(u2)
		if u1.InputTokens != 300 || u1.OutputTokens != 150 || u1.Requests != 3 {
			t.Errorf("After Add: %+v, want {Input:300, Output:150, Requests:3}", u1)
		}
	})

	t.Run("total tokens", func(t *testing.T) {
		u := Usage{InputTokens: 100, OutputTokens: 50}
		if got := u.TotalTokens(); got != 150 {
			t.Errorf("TotalTokens() = %d, want 150", got)
		}
	})
}

func TestWithTaskOverrides(t *testing.T) {
	overrides := map[any]ModelName{
		TaskImplement: ModelOpus,
		TaskSearch:    ModelHaiku,
	}

	s := NewSelector(WithTaskOverrides(overrides))

	if got := s.Select(TaskImplement); got != ModelOpus {
		t.Errorf("Select(TaskImplement) = %s, want %s", got, ModelOpus)
	}
	if got := s.Select(TaskSearch); got != ModelHaiku {
		t.Errorf("Select(TaskSearch) = %s, want %s", got, ModelHaiku)
	}
}
