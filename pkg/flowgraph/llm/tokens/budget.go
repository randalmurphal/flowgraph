package tokens

// DefaultSystemPercent is the default percentage for system prompts.
const DefaultSystemPercent = 20

// DefaultContextPercent is the default percentage for context.
const DefaultContextPercent = 40

// DefaultUserPercent is the default percentage for user messages.
const DefaultUserPercent = 30

// DefaultReservedPercent is the default percentage reserved for response.
const DefaultReservedPercent = 10

// Budget manages token allocation across prompt components.
type Budget struct {
	// Total is the total token budget available.
	Total int

	// System is the budget for system prompts.
	System int

	// Context is the budget for task context, history, etc.
	Context int

	// User is the budget for user messages.
	User int

	// Reserved is the budget reserved for response generation.
	Reserved int

	counter Counter
}

// NewBudget creates a budget with total tokens allocated proportionally.
// Default allocation: 20% system, 40% context, 30% user, 10% reserved.
func NewBudget(total int) *Budget {
	return &Budget{
		Total:    total,
		System:   total * DefaultSystemPercent / 100,
		Context:  total * DefaultContextPercent / 100,
		User:     total * DefaultUserPercent / 100,
		Reserved: total * DefaultReservedPercent / 100,
		counter:  NewEstimatingCounter(),
	}
}

// NewBudgetWithAllocation creates a budget with custom allocations.
// The allocations are specified as relative weights that are normalized
// to the total budget. For example, (100000, 20, 40, 30, 10) allocates
// 20% system, 40% context, 30% user, 10% reserved.
func NewBudgetWithAllocation(total, system, context, user, reserved int) *Budget {
	// Normalize allocations to fit total
	sum := system + context + user + reserved
	if sum == 0 {
		sum = 100
	}
	return &Budget{
		Total:    total,
		System:   total * system / sum,
		Context:  total * context / sum,
		User:     total * user / sum,
		Reserved: total * reserved / sum,
		counter:  NewEstimatingCounter(),
	}
}

// FitsSystem returns true if the system prompt fits within the system budget.
func (b *Budget) FitsSystem(text string) bool {
	return b.counter.FitsInLimit(text, b.System)
}

// FitsContext returns true if the context fits within the context budget.
func (b *Budget) FitsContext(text string) bool {
	return b.counter.FitsInLimit(text, b.Context)
}

// FitsUser returns true if the user message fits within the user budget.
func (b *Budget) FitsUser(text string) bool {
	return b.counter.FitsInLimit(text, b.User)
}

// FitsSystemTokens returns true if the token count fits within the system budget.
func (b *Budget) FitsSystemTokens(tokens int) bool {
	return tokens <= b.System
}

// FitsContextTokens returns true if the token count fits within the context budget.
func (b *Budget) FitsContextTokens(tokens int) bool {
	return tokens <= b.Context
}

// FitsUserTokens returns true if the token count fits within the user budget.
func (b *Budget) FitsUserTokens(tokens int) bool {
	return tokens <= b.User
}

// RemainingContext returns the remaining context budget after accounting for used tokens.
func (b *Budget) RemainingContext(usedTokens int) int {
	remaining := b.Context - usedTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// RemainingTotal returns remaining tokens after subtracting used amounts.
func (b *Budget) RemainingTotal(systemUsed, contextUsed, userUsed int) int {
	used := systemUsed + contextUsed + userUsed + b.Reserved
	remaining := b.Total - used
	if remaining < 0 {
		return 0
	}
	return remaining
}
