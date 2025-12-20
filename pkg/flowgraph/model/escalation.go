package model

// EscalationChain defines the order of models to try when escalating.
type EscalationChain struct {
	// Models in ascending order of capability (e.g., haiku, sonnet, opus)
	Models []ModelName

	// MaxAttempts is the maximum total attempts before giving up
	MaxAttempts int
}

// DefaultEscalation is the standard escalation chain.
var DefaultEscalation = EscalationChain{
	Models:      []ModelName{ModelSonnet, ModelOpus},
	MaxAttempts: 3,
}

// FullEscalation starts from haiku and goes through all tiers.
var FullEscalation = EscalationChain{
	Models:      []ModelName{ModelHaiku, ModelSonnet, ModelOpus},
	MaxAttempts: 5,
}

// NoEscalation disables model escalation (retry same model).
var NoEscalation = EscalationChain{
	Models:      nil,
	MaxAttempts: 3,
}

// Next returns the next model to try after a failure.
// Returns the next model in the chain and whether to continue.
// If already at the highest tier or max attempts reached, returns ("", false).
func (e *EscalationChain) Next(current ModelName, attempt int) (ModelName, bool) {
	// Check if we've exhausted attempts
	if attempt >= e.MaxAttempts {
		return "", false
	}

	// No escalation chain = retry same model
	if len(e.Models) == 0 {
		return current, true
	}

	// Find current model in chain
	idx := -1
	for i, m := range e.Models {
		if m == current {
			idx = i
			break
		}
	}

	// Model not in chain - start at beginning if any attempts left
	if idx < 0 {
		if len(e.Models) > 0 {
			return e.Models[0], true
		}
		return current, true
	}

	// Already at highest tier - stay there
	if idx >= len(e.Models)-1 {
		return current, true
	}

	// Escalate to next tier
	return e.Models[idx+1], true
}

// CanEscalate returns true if the current model can escalate to a higher tier.
func (e *EscalationChain) CanEscalate(current ModelName) bool {
	if len(e.Models) == 0 {
		return false
	}

	for i, m := range e.Models {
		if m == current {
			return i < len(e.Models)-1
		}
	}

	return false
}

// HighestModel returns the highest capability model in the chain.
func (e *EscalationChain) HighestModel() ModelName {
	if len(e.Models) == 0 {
		return ModelSonnet
	}
	return e.Models[len(e.Models)-1]
}

// EscalationState tracks the state of an escalation attempt.
type EscalationState struct {
	Chain        *EscalationChain
	CurrentModel ModelName
	Attempt      int
	LastError    error
}

// NewEscalationState creates a new escalation state starting at the given model.
func NewEscalationState(chain *EscalationChain, startModel ModelName) *EscalationState {
	if chain == nil {
		chain = &DefaultEscalation
	}
	return &EscalationState{
		Chain:        chain,
		CurrentModel: startModel,
		Attempt:      0,
	}
}

// RecordFailure records a failed attempt and escalates if possible.
// Returns true if escalation occurred and there are more attempts available.
func (s *EscalationState) RecordFailure(err error) bool {
	s.Attempt++
	s.LastError = err

	next, ok := s.Chain.Next(s.CurrentModel, s.Attempt)
	if !ok {
		return false
	}

	if next != s.CurrentModel {
		s.CurrentModel = next
	}
	return true
}

// Exhausted returns true if all attempts have been used.
func (s *EscalationState) Exhausted() bool {
	return s.Attempt >= s.Chain.MaxAttempts
}
