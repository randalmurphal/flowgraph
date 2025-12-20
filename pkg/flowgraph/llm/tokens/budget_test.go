package tokens

import (
	"strings"
	"testing"
)

func TestNewBudget(t *testing.T) {
	total := 100000
	b := NewBudget(total)

	if b.Total != total {
		t.Errorf("expected Total %d, got %d", total, b.Total)
	}
	if b.System != 20000 {
		t.Errorf("expected System 20000, got %d", b.System)
	}
	if b.Context != 40000 {
		t.Errorf("expected Context 40000, got %d", b.Context)
	}
	if b.User != 30000 {
		t.Errorf("expected User 30000, got %d", b.User)
	}
	if b.Reserved != 10000 {
		t.Errorf("expected Reserved 10000, got %d", b.Reserved)
	}
	if b.counter == nil {
		t.Error("expected counter to be initialized")
	}
}

func TestNewBudget_SmallTotal(t *testing.T) {
	// Test with small total to check integer division
	b := NewBudget(100)

	if b.Total != 100 {
		t.Errorf("expected Total 100, got %d", b.Total)
	}
	if b.System != 20 {
		t.Errorf("expected System 20, got %d", b.System)
	}
	if b.Context != 40 {
		t.Errorf("expected Context 40, got %d", b.Context)
	}
	if b.User != 30 {
		t.Errorf("expected User 30, got %d", b.User)
	}
	if b.Reserved != 10 {
		t.Errorf("expected Reserved 10, got %d", b.Reserved)
	}
}

func TestNewBudgetWithAllocation(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		system   int
		context  int
		user     int
		reserved int
		expected Budget
	}{
		{
			name:     "equal allocation",
			total:    100000,
			system:   25,
			context:  25,
			user:     25,
			reserved: 25,
			expected: Budget{
				Total:    100000,
				System:   25000,
				Context:  25000,
				User:     25000,
				Reserved: 25000,
			},
		},
		{
			name:     "heavy context",
			total:    100000,
			system:   10,
			context:  60,
			user:     20,
			reserved: 10,
			expected: Budget{
				Total:    100000,
				System:   10000,
				Context:  60000,
				User:     20000,
				Reserved: 10000,
			},
		},
		{
			name:     "no reserved",
			total:    100000,
			system:   30,
			context:  50,
			user:     20,
			reserved: 0,
			expected: Budget{
				Total:    100000,
				System:   30000,
				Context:  50000,
				User:     20000,
				Reserved: 0,
			},
		},
		{
			name:     "all zeros uses default sum",
			total:    100000,
			system:   0,
			context:  0,
			user:     0,
			reserved: 0,
			expected: Budget{
				Total:    100000,
				System:   0,
				Context:  0,
				User:     0,
				Reserved: 0,
			},
		},
		{
			name:     "non-100 sum is normalized",
			total:    100000,
			system:   10,
			context:  20,
			user:     15,
			reserved: 5, // sum = 50
			expected: Budget{
				Total:    100000,
				System:   20000,
				Context:  40000,
				User:     30000,
				Reserved: 10000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBudgetWithAllocation(tt.total, tt.system, tt.context, tt.user, tt.reserved)

			if b.Total != tt.expected.Total {
				t.Errorf("Total = %d, expected %d", b.Total, tt.expected.Total)
			}
			if b.System != tt.expected.System {
				t.Errorf("System = %d, expected %d", b.System, tt.expected.System)
			}
			if b.Context != tt.expected.Context {
				t.Errorf("Context = %d, expected %d", b.Context, tt.expected.Context)
			}
			if b.User != tt.expected.User {
				t.Errorf("User = %d, expected %d", b.User, tt.expected.User)
			}
			if b.Reserved != tt.expected.Reserved {
				t.Errorf("Reserved = %d, expected %d", b.Reserved, tt.expected.Reserved)
			}
			if b.counter == nil {
				t.Error("counter should be initialized")
			}
		})
	}
}

func TestBudget_FitsSystem(t *testing.T) {
	b := NewBudget(100000) // System = 20000 tokens

	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "empty fits",
			text:     "",
			expected: true,
		},
		{
			name:     "short text fits",
			text:     "You are a helpful assistant.",
			expected: true,
		},
		{
			name:     "long text does not fit",
			text:     strings.Repeat("x", 100000), // ~25000 tokens
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.FitsSystem(tt.text)
			if result != tt.expected {
				t.Errorf("FitsSystem() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestBudget_FitsContext(t *testing.T) {
	b := NewBudget(100000) // Context = 40000 tokens

	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "empty fits",
			text:     "",
			expected: true,
		},
		{
			name:     "normal context fits",
			text:     strings.Repeat("context ", 10000), // ~20000 tokens
			expected: true,
		},
		{
			name:     "huge context does not fit",
			text:     strings.Repeat("x", 200000), // ~50000 tokens
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.FitsContext(tt.text)
			if result != tt.expected {
				t.Errorf("FitsContext() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestBudget_FitsUser(t *testing.T) {
	b := NewBudget(100000) // User = 30000 tokens

	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "empty fits",
			text:     "",
			expected: true,
		},
		{
			name:     "normal user message fits",
			text:     "Please help me with this task.",
			expected: true,
		},
		{
			name:     "huge user message does not fit",
			text:     strings.Repeat("x", 150000), // ~37500 tokens
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.FitsUser(tt.text)
			if result != tt.expected {
				t.Errorf("FitsUser() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestBudget_FitsSystemTokens(t *testing.T) {
	b := NewBudget(100000) // System = 20000 tokens

	tests := []struct {
		name     string
		tokens   int
		expected bool
	}{
		{name: "zero fits", tokens: 0, expected: true},
		{name: "within limit fits", tokens: 10000, expected: true},
		{name: "exact limit fits", tokens: 20000, expected: true},
		{name: "over limit does not fit", tokens: 25000, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.FitsSystemTokens(tt.tokens)
			if result != tt.expected {
				t.Errorf("FitsSystemTokens(%d) = %v, expected %v", tt.tokens, result, tt.expected)
			}
		})
	}
}

func TestBudget_FitsContextTokens(t *testing.T) {
	b := NewBudget(100000) // Context = 40000 tokens

	tests := []struct {
		name     string
		tokens   int
		expected bool
	}{
		{name: "zero fits", tokens: 0, expected: true},
		{name: "within limit fits", tokens: 30000, expected: true},
		{name: "exact limit fits", tokens: 40000, expected: true},
		{name: "over limit does not fit", tokens: 50000, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.FitsContextTokens(tt.tokens)
			if result != tt.expected {
				t.Errorf("FitsContextTokens(%d) = %v, expected %v", tt.tokens, result, tt.expected)
			}
		})
	}
}

func TestBudget_FitsUserTokens(t *testing.T) {
	b := NewBudget(100000) // User = 30000 tokens

	tests := []struct {
		name     string
		tokens   int
		expected bool
	}{
		{name: "zero fits", tokens: 0, expected: true},
		{name: "within limit fits", tokens: 20000, expected: true},
		{name: "exact limit fits", tokens: 30000, expected: true},
		{name: "over limit does not fit", tokens: 35000, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.FitsUserTokens(tt.tokens)
			if result != tt.expected {
				t.Errorf("FitsUserTokens(%d) = %v, expected %v", tt.tokens, result, tt.expected)
			}
		})
	}
}

func TestBudget_RemainingContext(t *testing.T) {
	b := NewBudget(100000) // Context = 40000 tokens

	tests := []struct {
		name       string
		usedTokens int
		expected   int
	}{
		{
			name:       "none used",
			usedTokens: 0,
			expected:   40000,
		},
		{
			name:       "some used",
			usedTokens: 10000,
			expected:   30000,
		},
		{
			name:       "all used",
			usedTokens: 40000,
			expected:   0,
		},
		{
			name:       "over budget returns zero",
			usedTokens: 50000,
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.RemainingContext(tt.usedTokens)
			if result != tt.expected {
				t.Errorf("RemainingContext(%d) = %d, expected %d",
					tt.usedTokens, result, tt.expected)
			}
		})
	}
}

func TestBudget_RemainingTotal(t *testing.T) {
	b := NewBudget(100000)
	// System=20000, Context=40000, User=30000, Reserved=10000

	tests := []struct {
		name        string
		systemUsed  int
		contextUsed int
		userUsed    int
		expected    int
	}{
		{
			name:        "nothing used",
			systemUsed:  0,
			contextUsed: 0,
			userUsed:    0,
			expected:    90000, // Total minus Reserved
		},
		{
			name:        "some used",
			systemUsed:  5000,
			contextUsed: 10000,
			userUsed:    5000,
			expected:    70000,
		},
		{
			name:        "all allocated used",
			systemUsed:  20000,
			contextUsed: 40000,
			userUsed:    30000,
			expected:    0,
		},
		{
			name:        "over budget returns zero",
			systemUsed:  30000,
			contextUsed: 50000,
			userUsed:    40000,
			expected:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.RemainingTotal(tt.systemUsed, tt.contextUsed, tt.userUsed)
			if result != tt.expected {
				t.Errorf("RemainingTotal(%d, %d, %d) = %d, expected %d",
					tt.systemUsed, tt.contextUsed, tt.userUsed, result, tt.expected)
			}
		})
	}
}

func TestBudget_AllocationsSumCorrectly(t *testing.T) {
	totals := []int{100000, 200000, 50000, 1000}

	for _, total := range totals {
		t.Run("", func(t *testing.T) {
			b := NewBudget(total)
			sum := b.System + b.Context + b.User + b.Reserved

			if sum != total {
				t.Errorf("allocations sum to %d, expected %d", sum, total)
			}
		})
	}
}

func TestBudget_DefaultConstants(t *testing.T) {
	// Verify default constants add up to 100
	sum := DefaultSystemPercent + DefaultContextPercent + DefaultUserPercent + DefaultReservedPercent
	if sum != 100 {
		t.Errorf("default percentages sum to %d, expected 100", sum)
	}
}

func BenchmarkNewBudget(b *testing.B) {
	for range b.N {
		NewBudget(100000)
	}
}

func BenchmarkBudget_FitsContext(b *testing.B) {
	budget := NewBudget(100000)
	text := strings.Repeat("context data ", 100)

	b.ResetTimer()
	for range b.N {
		budget.FitsContext(text)
	}
}

func BenchmarkBudget_RemainingTotal(b *testing.B) {
	budget := NewBudget(100000)

	b.ResetTimer()
	for range b.N {
		budget.RemainingTotal(5000, 10000, 5000)
	}
}
