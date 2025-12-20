package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExpand_BraceStyle tests ${var} pattern expansion.
func TestExpand_BraceStyle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		vars     map[string]any
		expected string
	}{
		{
			name:     "simple variable",
			input:    "Hello ${name}",
			vars:     map[string]any{"name": "World"},
			expected: "Hello World",
		},
		{
			name:     "multiple variables",
			input:    "${greeting} ${name}!",
			vars:     map[string]any{"greeting": "Hello", "name": "World"},
			expected: "Hello World!",
		},
		{
			name:     "variable at start",
			input:    "${prefix}-suffix",
			vars:     map[string]any{"prefix": "test"},
			expected: "test-suffix",
		},
		{
			name:     "variable at end",
			input:    "prefix-${suffix}",
			vars:     map[string]any{"suffix": "test"},
			expected: "prefix-test",
		},
		{
			name:     "adjacent variables",
			input:    "${a}${b}${c}",
			vars:     map[string]any{"a": "1", "b": "2", "c": "3"},
			expected: "123",
		},
		{
			name:     "numeric value",
			input:    "port: ${port}",
			vars:     map[string]any{"port": 8080},
			expected: "port: 8080",
		},
		{
			name:     "boolean value",
			input:    "enabled: ${enabled}",
			vars:     map[string]any{"enabled": true},
			expected: "enabled: true",
		},
		{
			name:     "underscore in variable name",
			input:    "${my_var}",
			vars:     map[string]any{"my_var": "value"},
			expected: "value",
		},
		{
			name:     "number in variable name",
			input:    "${var1}",
			vars:     map[string]any{"var1": "value"},
			expected: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Expand(tt.input, tt.vars)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExpand_DollarStyle tests $var pattern expansion.
func TestExpand_DollarStyle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		vars     map[string]any
		expected string
	}{
		{
			name:     "simple variable",
			input:    "Hello $name",
			vars:     map[string]any{"name": "World"},
			expected: "Hello World",
		},
		{
			name:     "variable at end of string",
			input:    "Hello $name",
			vars:     map[string]any{"name": "World"},
			expected: "Hello World",
		},
		{
			name:     "variable followed by space",
			input:    "$greeting friend",
			vars:     map[string]any{"greeting": "Hello"},
			expected: "Hello friend",
		},
		{
			name:     "variable followed by punctuation",
			input:    "$name!",
			vars:     map[string]any{"name": "World"},
			expected: "World!",
		},
		{
			name:     "word boundary detection",
			input:    "$port is different from $portNumber",
			vars:     map[string]any{"port": "8080", "portNumber": "9090"},
			expected: "8080 is different from 9090",
		},
		{
			name:     "multiple dollar variables",
			input:    "$a $b $c",
			vars:     map[string]any{"a": "1", "b": "2", "c": "3"},
			expected: "1 2 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Expand(tt.input, tt.vars)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExpand_MixedStyles tests both ${var} and $var in same string.
func TestExpand_MixedStyles(t *testing.T) {
	vars := map[string]any{
		"host": "api.example.com",
		"port": 8080,
	}

	result := Expand("https://${host}:$port/api", vars)
	assert.Equal(t, "https://api.example.com:8080/api", result)
}

// TestExpand_MissingVariables tests behavior with missing variables.
func TestExpand_MissingVariables(t *testing.T) {
	t.Run("MissingKeep keeps placeholder", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingKeep))
		result, err := exp.Expand("Hello ${missing}", nil)
		require.NoError(t, err)
		assert.Equal(t, "Hello ${missing}", result)
	})

	t.Run("MissingKeep keeps dollar placeholder", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingKeep))
		result, err := exp.Expand("Hello $missing", nil)
		require.NoError(t, err)
		assert.Equal(t, "Hello $missing", result)
	})

	t.Run("MissingEmpty replaces with empty string", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingEmpty))
		result, err := exp.Expand("Hello ${missing}!", nil)
		require.NoError(t, err)
		assert.Equal(t, "Hello !", result)
	})

	t.Run("MissingEmpty replaces dollar with empty string", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingEmpty))
		result, err := exp.Expand("Hello $missing!", nil)
		require.NoError(t, err)
		assert.Equal(t, "Hello !", result)
	})

	t.Run("MissingError returns error for brace style", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingError))
		_, err := exp.Expand("Hello ${missing}", nil)
		require.Error(t, err)

		var undefinedErr *UndefinedVariableError
		require.ErrorAs(t, err, &undefinedErr)
		assert.Equal(t, []string{"missing"}, undefinedErr.Names)
		assert.Equal(t, "undefined variable: missing", err.Error())
	})

	t.Run("MissingError returns error for dollar style", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingError))
		_, err := exp.Expand("Hello $missing", nil)
		require.Error(t, err)

		var undefinedErr *UndefinedVariableError
		require.ErrorAs(t, err, &undefinedErr)
		assert.Equal(t, []string{"missing"}, undefinedErr.Names)
	})

	t.Run("MissingError with multiple missing", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingError))
		_, err := exp.Expand("${a} ${b} $c", nil)
		require.Error(t, err)

		var undefinedErr *UndefinedVariableError
		require.ErrorAs(t, err, &undefinedErr)
		assert.Len(t, undefinedErr.Names, 3)
		assert.Contains(t, err.Error(), "undefined variables:")
	})

	t.Run("partial variables found", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingError))
		_, err := exp.Expand("${found} ${missing}", map[string]any{"found": "yes"})
		require.Error(t, err)

		var undefinedErr *UndefinedVariableError
		require.ErrorAs(t, err, &undefinedErr)
		assert.Equal(t, []string{"missing"}, undefinedErr.Names)
	})
}

// TestExpand_EdgeCases tests edge cases.
func TestExpand_EdgeCases(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		result := Expand("", map[string]any{"a": "b"})
		assert.Equal(t, "", result)
	})

	t.Run("nil vars", func(t *testing.T) {
		result := Expand("Hello ${name}", nil)
		assert.Equal(t, "Hello ${name}", result)
	})

	t.Run("empty vars", func(t *testing.T) {
		result := Expand("Hello ${name}", map[string]any{})
		assert.Equal(t, "Hello ${name}", result)
	})

	t.Run("no variables in string", func(t *testing.T) {
		result := Expand("Hello World", map[string]any{"name": "value"})
		assert.Equal(t, "Hello World", result)
	})

	t.Run("dollar sign without variable", func(t *testing.T) {
		result := Expand("$100 dollars", map[string]any{})
		// $100 should not be treated as a variable (starts with digit)
		assert.Equal(t, "$100 dollars", result)
	})

	t.Run("empty braces", func(t *testing.T) {
		// ${} is not a valid variable pattern
		result := Expand("${}", map[string]any{})
		assert.Equal(t, "${}", result)
	})

	t.Run("nested braces should not recursively expand", func(t *testing.T) {
		// ${${inner}} - the inner $inner gets expanded, but result is not re-expanded
		// First: $inner -> name, giving ${name}
		// But ${name} inside a malformed brace pattern stays as-is
		// Actually: the brace pattern regex only matches ${varname} where varname
		// is alphanumeric. So ${${inner}} doesn't match brace pattern.
		// But $inner DOES match dollar pattern, so we get ${name}.
		result := Expand("${${inner}}", map[string]any{"inner": "name", "name": "value"})
		assert.Equal(t, "${name}", result) // $inner expanded, but no recursive expansion
	})

	t.Run("escaped-like patterns", func(t *testing.T) {
		// $$var is not a special escape
		result := Expand("$$var", map[string]any{"var": "value"})
		assert.Equal(t, "$value", result)
	})

	t.Run("variable with only underscore", func(t *testing.T) {
		result := Expand("${_}", map[string]any{"_": "underscore"})
		assert.Equal(t, "underscore", result)
	})

	t.Run("variable starting with underscore", func(t *testing.T) {
		result := Expand("${_private}", map[string]any{"_private": "secret"})
		assert.Equal(t, "secret", result)
	})
}

// TestExpand_DisabledStyles tests disabling pattern styles.
func TestExpand_DisabledStyles(t *testing.T) {
	vars := map[string]any{"name": "World"}

	t.Run("disable brace style", func(t *testing.T) {
		exp := NewExpander(WithBraceStyle(false))
		result, err := exp.Expand("${name} $name", vars)
		require.NoError(t, err)
		assert.Equal(t, "${name} World", result)
	})

	t.Run("disable dollar style", func(t *testing.T) {
		exp := NewExpander(WithDollarStyle(false))
		result, err := exp.Expand("${name} $name", vars)
		require.NoError(t, err)
		assert.Equal(t, "World $name", result)
	})

	t.Run("disable both styles", func(t *testing.T) {
		exp := NewExpander(WithBraceStyle(false), WithDollarStyle(false))
		result, err := exp.Expand("${name} $name", vars)
		require.NoError(t, err)
		assert.Equal(t, "${name} $name", result)
	})
}

// TestMustExpand tests the MustExpand method.
func TestMustExpand(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		exp := NewExpander()
		result := exp.MustExpand("Hello ${name}", map[string]any{"name": "World"})
		assert.Equal(t, "Hello World", result)
	})

	t.Run("panics on error", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingError))
		assert.Panics(t, func() {
			exp.MustExpand("${missing}", nil)
		})
	})
}

// TestExpandAll tests batch expansion of string slices.
func TestExpandAll(t *testing.T) {
	vars := map[string]any{"env": "prod", "region": "us-east"}

	t.Run("basic expansion", func(t *testing.T) {
		input := []string{
			"https://${env}.api.com",
			"https://${env}.${region}.db.com",
		}
		result := ExpandAll(input, vars)
		assert.Equal(t, []string{
			"https://prod.api.com",
			"https://prod.us-east.db.com",
		}, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		result := ExpandAll(nil, vars)
		assert.Nil(t, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		result := ExpandAll([]string{}, vars)
		assert.Equal(t, []string{}, result)
	})

	t.Run("expander with error", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingError))
		_, err := exp.ExpandAll([]string{"${missing}"}, nil)
		require.Error(t, err)
	})
}

// TestExpandMap tests recursive map expansion.
func TestExpandMap(t *testing.T) {
	vars := map[string]any{"env": "prod", "host": "api.example.com"}

	t.Run("basic map expansion", func(t *testing.T) {
		input := map[string]any{
			"url":  "https://${host}/api",
			"name": "${env}",
		}
		result := ExpandMap(input, vars)
		assert.Equal(t, map[string]any{
			"url":  "https://api.example.com/api",
			"name": "prod",
		}, result)
	})

	t.Run("non-string values preserved", func(t *testing.T) {
		input := map[string]any{
			"url":     "https://${host}",
			"port":    8080,
			"enabled": true,
			"count":   int64(42),
		}
		result := ExpandMap(input, vars)
		assert.Equal(t, "https://api.example.com", result["url"])
		assert.Equal(t, 8080, result["port"])
		assert.Equal(t, true, result["enabled"])
		assert.Equal(t, int64(42), result["count"])
	})

	t.Run("nested map expansion", func(t *testing.T) {
		input := map[string]any{
			"top": "${env}",
			"nested": map[string]any{
				"url": "https://${host}/api/${env}",
				"deep": map[string]any{
					"value": "$env",
				},
			},
		}
		result := ExpandMap(input, vars)
		assert.Equal(t, "prod", result["top"])

		nested, ok := result["nested"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "https://api.example.com/api/prod", nested["url"])

		deep, ok := nested["deep"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "prod", deep["value"])
	})

	t.Run("nil map", func(t *testing.T) {
		result := ExpandMap(nil, vars)
		assert.Nil(t, result)
	})

	t.Run("empty map", func(t *testing.T) {
		result := ExpandMap(map[string]any{}, vars)
		assert.Equal(t, map[string]any{}, result)
	})

	t.Run("expander with error", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingError))
		_, err := exp.ExpandMap(map[string]any{"key": "${missing}"}, nil)
		require.Error(t, err)
	})

	t.Run("error in nested map", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingError))
		_, err := exp.ExpandMap(map[string]any{
			"nested": map[string]any{
				"key": "${missing}",
			},
		}, nil)
		require.Error(t, err)
	})
}

// TestNewExpander tests expander creation with options.
func TestNewExpander(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		exp := NewExpander()
		assert.Equal(t, MissingKeep, exp.missingAction)
		assert.True(t, exp.braceStyle)
		assert.True(t, exp.dollarStyle)
	})

	t.Run("custom missing action", func(t *testing.T) {
		exp := NewExpander(WithMissingAction(MissingError))
		assert.Equal(t, MissingError, exp.missingAction)
	})

	t.Run("multiple options", func(t *testing.T) {
		exp := NewExpander(
			WithMissingAction(MissingEmpty),
			WithBraceStyle(false),
			WithDollarStyle(true),
		)
		assert.Equal(t, MissingEmpty, exp.missingAction)
		assert.False(t, exp.braceStyle)
		assert.True(t, exp.dollarStyle)
	})
}

// TestUndefinedVariableError tests error formatting.
func TestUndefinedVariableError(t *testing.T) {
	t.Run("single variable", func(t *testing.T) {
		err := &UndefinedVariableError{Names: []string{"foo"}}
		assert.Equal(t, "undefined variable: foo", err.Error())
	})

	t.Run("multiple variables", func(t *testing.T) {
		err := &UndefinedVariableError{Names: []string{"foo", "bar", "baz"}}
		assert.Equal(t, "undefined variables: foo, bar, baz", err.Error())
	})
}

// TestPackageLevelFunctions tests the convenience functions.
func TestPackageLevelFunctions(t *testing.T) {
	vars := map[string]any{"name": "World", "greeting": "Hello"}

	t.Run("Expand", func(t *testing.T) {
		result := Expand("${greeting} ${name}", vars)
		assert.Equal(t, "Hello World", result)
	})

	t.Run("ExpandAll", func(t *testing.T) {
		result := ExpandAll([]string{"${greeting}", "${name}"}, vars)
		assert.Equal(t, []string{"Hello", "World"}, result)
	})

	t.Run("ExpandMap", func(t *testing.T) {
		result := ExpandMap(map[string]any{"msg": "${greeting} ${name}"}, vars)
		assert.Equal(t, "Hello World", result["msg"])
	})
}

// TestExpand_RealWorldScenarios tests realistic use cases.
func TestExpand_RealWorldScenarios(t *testing.T) {
	t.Run("URL construction", func(t *testing.T) {
		vars := map[string]any{
			"protocol": "https",
			"host":     "api.example.com",
			"port":     443,
			"version":  "v1",
			"resource": "users",
		}
		url := Expand("${protocol}://${host}:${port}/api/${version}/${resource}", vars)
		assert.Equal(t, "https://api.example.com:443/api/v1/users", url)
	})

	t.Run("database connection string", func(t *testing.T) {
		vars := map[string]any{
			"user":     "admin",
			"password": "secret123",
			"host":     "localhost",
			"port":     5432,
			"dbname":   "myapp",
		}
		connStr := Expand("postgres://${user}:${password}@${host}:${port}/${dbname}", vars)
		assert.Equal(t, "postgres://admin:secret123@localhost:5432/myapp", connStr)
	})

	t.Run("log message template", func(t *testing.T) {
		vars := map[string]any{
			"service": "auth",
			"method":  "login",
			"user_id": "user-123",
		}
		msg := Expand("[$service] $method called by $user_id", vars)
		assert.Equal(t, "[auth] login called by user-123", msg)
	})

	t.Run("kubernetes manifest values", func(t *testing.T) {
		vars := map[string]any{
			"namespace": "production",
			"app_name":  "myapp",
			"replicas":  3,
			"image":     "myrepo/myapp:v1.2.3",
		}
		config := ExpandMap(map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "${app_name}",
				"namespace": "${namespace}",
			},
			"spec": map[string]any{
				"replicas": "${replicas}",
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name":  "${app_name}",
								"image": "${image}",
							},
						},
					},
				},
			},
		}, vars)

		metadata := config["metadata"].(map[string]any)
		assert.Equal(t, "myapp", metadata["name"])
		assert.Equal(t, "production", metadata["namespace"])
	})
}
