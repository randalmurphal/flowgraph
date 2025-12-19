package flowgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWithMaxIterations_Valid tests valid max iterations values.
func TestWithMaxIterations_Valid(t *testing.T) {
	tests := []struct {
		name  string
		value int
	}{
		{"minimum valid", 1},
		{"typical value", 100},
		{"default value", DefaultMaxIterations},
		{"large value", 50000},
		{"maximum valid", MaxIterationsLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				opt := WithMaxIterations(tt.value)
				cfg := defaultRunConfig()
				opt(&cfg)
				assert.Equal(t, tt.value, cfg.maxIterations)
			})
		})
	}
}

// TestWithMaxIterations_PanicsOnZero tests panic for zero value.
func TestWithMaxIterations_PanicsOnZero(t *testing.T) {
	assert.PanicsWithValue(t, "flowgraph: max iterations must be > 0", func() {
		WithMaxIterations(0)
	})
}

// TestWithMaxIterations_PanicsOnNegative tests panic for negative values.
func TestWithMaxIterations_PanicsOnNegative(t *testing.T) {
	assert.PanicsWithValue(t, "flowgraph: max iterations must be > 0", func() {
		WithMaxIterations(-1)
	})

	assert.PanicsWithValue(t, "flowgraph: max iterations must be > 0", func() {
		WithMaxIterations(-100)
	})
}

// TestWithMaxIterations_PanicsOnExceedingLimit tests panic for values exceeding limit.
func TestWithMaxIterations_PanicsOnExceedingLimit(t *testing.T) {
	assert.PanicsWithValue(t, "flowgraph: max iterations exceeds limit (100000)", func() {
		WithMaxIterations(MaxIterationsLimit + 1)
	})

	assert.PanicsWithValue(t, "flowgraph: max iterations exceeds limit (100000)", func() {
		WithMaxIterations(1000000)
	})
}

// TestDefaultMaxIterations_Constant tests the default constant value.
func TestDefaultMaxIterations_Constant(t *testing.T) {
	assert.Equal(t, 1000, DefaultMaxIterations)
	assert.Equal(t, 100000, MaxIterationsLimit)
}
