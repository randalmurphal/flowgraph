package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew verifies Config creation from maps.
func TestNew(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{"nil map", nil},
		{"empty map", map[string]any{}},
		{"with values", map[string]any{"key": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New(tt.data)
			assert.NotNil(t, cfg.Raw())
		})
	}
}

// TestString verifies string extraction with defaults.
func TestString(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		key        string
		defaultVal string
		want       string
	}{
		{"key exists", map[string]any{"name": "alice"}, "name", "default", "alice"},
		{"key missing", map[string]any{"other": "value"}, "name", "default", "default"},
		{"empty string", map[string]any{"name": ""}, "name", "default", ""},
		{"wrong type int", map[string]any{"name": 123}, "name", "default", "default"},
		{"wrong type bool", map[string]any{"name": true}, "name", "default", "default"},
		{"wrong type slice", map[string]any{"name": []string{"a"}}, "name", "default", "default"},
		{"nil map", nil, "name", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New(tt.data)
			got := cfg.String(tt.key, tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDuration verifies duration extraction with various input types.
func TestDuration(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		key        string
		defaultVal time.Duration
		want       time.Duration
	}{
		{
			"string duration",
			map[string]any{"timeout": "30s"},
			"timeout",
			10 * time.Second,
			30 * time.Second,
		},
		{
			"string complex duration",
			map[string]any{"timeout": "1h30m"},
			"timeout",
			10 * time.Second,
			90 * time.Minute,
		},
		{
			"int seconds",
			map[string]any{"timeout": 60},
			"timeout",
			10 * time.Second,
			60 * time.Second,
		},
		{
			"int64 seconds",
			map[string]any{"timeout": int64(45)},
			"timeout",
			10 * time.Second,
			45 * time.Second,
		},
		{
			"float64 seconds",
			map[string]any{"timeout": 30.5},
			"timeout",
			10 * time.Second,
			30*time.Second + 500*time.Millisecond,
		},
		{
			"time.Duration directly",
			map[string]any{"timeout": 5 * time.Minute},
			"timeout",
			10 * time.Second,
			5 * time.Minute,
		},
		{
			"key missing",
			map[string]any{"other": "value"},
			"timeout",
			10 * time.Second,
			10 * time.Second,
		},
		{
			"invalid string",
			map[string]any{"timeout": "invalid"},
			"timeout",
			10 * time.Second,
			10 * time.Second,
		},
		{
			"wrong type bool",
			map[string]any{"timeout": true},
			"timeout",
			10 * time.Second,
			10 * time.Second,
		},
		{
			"nil map",
			nil,
			"timeout",
			10 * time.Second,
			10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New(tt.data)
			got := cfg.Duration(tt.key, tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestBool verifies boolean extraction with defaults.
func TestBool(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		key        string
		defaultVal bool
		want       bool
	}{
		{"true value", map[string]any{"enabled": true}, "enabled", false, true},
		{"false value", map[string]any{"enabled": false}, "enabled", true, false},
		{"key missing default false", map[string]any{"other": true}, "enabled", false, false},
		{"key missing default true", map[string]any{"other": false}, "enabled", true, true},
		{"wrong type string", map[string]any{"enabled": "true"}, "enabled", false, false},
		{"wrong type int", map[string]any{"enabled": 1}, "enabled", false, false},
		{"nil map", nil, "enabled", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New(tt.data)
			got := cfg.Bool(tt.key, tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestInt verifies integer extraction with type coercion.
func TestInt(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		key        string
		defaultVal int
		want       int
	}{
		{"int value", map[string]any{"count": 42}, "count", 0, 42},
		{"int64 value", map[string]any{"count": int64(100)}, "count", 0, 100},
		{"float64 whole", map[string]any{"count": 50.0}, "count", 0, 50},
		{"float64 fractional", map[string]any{"count": 50.5}, "count", 99, 99},
		{"key missing", map[string]any{"other": 1}, "count", 99, 99},
		{"wrong type string", map[string]any{"count": "42"}, "count", 99, 99},
		{"wrong type bool", map[string]any{"count": true}, "count", 99, 99},
		{"negative int", map[string]any{"count": -5}, "count", 0, -5},
		{"zero", map[string]any{"count": 0}, "count", 99, 0},
		{"nil map", nil, "count", 99, 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New(tt.data)
			got := cfg.Int(tt.key, tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFloat verifies float64 extraction with type coercion.
func TestFloat(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		key        string
		defaultVal float64
		want       float64
	}{
		{"float64 value", map[string]any{"rate": 3.14}, "rate", 0.0, 3.14},
		{"int value", map[string]any{"rate": 42}, "rate", 0.0, 42.0},
		{"int64 value", map[string]any{"rate": int64(100)}, "rate", 0.0, 100.0},
		{"key missing", map[string]any{"other": 1.0}, "rate", 9.99, 9.99},
		{"wrong type string", map[string]any{"rate": "3.14"}, "rate", 9.99, 9.99},
		{"wrong type bool", map[string]any{"rate": true}, "rate", 9.99, 9.99},
		{"negative float", map[string]any{"rate": -2.5}, "rate", 0.0, -2.5},
		{"zero", map[string]any{"rate": 0.0}, "rate", 9.99, 0.0},
		{"nil map", nil, "rate", 9.99, 9.99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New(tt.data)
			got := cfg.Float(tt.key, tt.defaultVal)
			assert.InDelta(t, tt.want, got, 0.001)
		})
	}
}

// TestStringSlice verifies string slice extraction.
func TestStringSlice(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		key        string
		defaultVal []string
		want       []string
	}{
		{
			"[]string value",
			map[string]any{"tags": []string{"a", "b", "c"}},
			"tags",
			[]string{"default"},
			[]string{"a", "b", "c"},
		},
		{
			"[]any with strings",
			map[string]any{"tags": []any{"x", "y", "z"}},
			"tags",
			[]string{"default"},
			[]string{"x", "y", "z"},
		},
		{
			"[]any with mixed types",
			map[string]any{"tags": []any{"a", 123, "b"}},
			"tags",
			[]string{"default"},
			[]string{"default"},
		},
		{
			"empty slice",
			map[string]any{"tags": []string{}},
			"tags",
			[]string{"default"},
			[]string{},
		},
		{
			"empty []any",
			map[string]any{"tags": []any{}},
			"tags",
			[]string{"default"},
			[]string{},
		},
		{
			"key missing",
			map[string]any{"other": []string{"a"}},
			"tags",
			[]string{"default"},
			[]string{"default"},
		},
		{
			"wrong type string",
			map[string]any{"tags": "not-a-slice"},
			"tags",
			[]string{"default"},
			[]string{"default"},
		},
		{
			"wrong type int slice",
			map[string]any{"tags": []int{1, 2, 3}},
			"tags",
			[]string{"default"},
			[]string{"default"},
		},
		{
			"nil default",
			map[string]any{"other": "value"},
			"tags",
			nil,
			nil,
		},
		{
			"nil map",
			nil,
			"tags",
			[]string{"default"},
			[]string{"default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New(tt.data)
			got := cfg.StringSlice(tt.key, tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAny verifies raw value extraction.
func TestAny(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		key        string
		defaultVal any
		want       any
	}{
		{"string value", map[string]any{"val": "hello"}, "val", nil, "hello"},
		{"int value", map[string]any{"val": 42}, "val", nil, 42},
		{"bool value", map[string]any{"val": true}, "val", nil, true},
		{"slice value", map[string]any{"val": []int{1, 2}}, "val", nil, []int{1, 2}},
		{"map value", map[string]any{"val": map[string]int{"a": 1}}, "val", nil, map[string]int{"a": 1}},
		{"key missing", map[string]any{"other": 1}, "val", "default", "default"},
		{"nil value", map[string]any{"val": nil}, "val", "default", nil},
		{"nil map", nil, "val", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New(tt.data)
			got := cfg.Any(tt.key, tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestHas verifies key existence check.
func TestHas(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		key  string
		want bool
	}{
		{"key exists", map[string]any{"name": "alice"}, "name", true},
		{"key missing", map[string]any{"other": "value"}, "name", false},
		{"nil value exists", map[string]any{"name": nil}, "name", true},
		{"empty map", map[string]any{}, "name", false},
		{"nil map", nil, "name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New(tt.data)
			got := cfg.Has(tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestRaw verifies access to underlying map.
func TestRaw(t *testing.T) {
	data := map[string]any{"key": "value"}
	cfg := config.New(data)

	raw := cfg.Raw()
	assert.Equal(t, data, raw)
}

// TestFromYAML verifies YAML parsing.
func TestFromYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		check   func(*testing.T, config.Config)
	}{
		{
			"simple values",
			`name: alice
count: 42
enabled: true`,
			false,
			func(t *testing.T, cfg config.Config) {
				assert.Equal(t, "alice", cfg.String("name", ""))
				assert.Equal(t, 42, cfg.Int("count", 0))
				assert.True(t, cfg.Bool("enabled", false))
			},
		},
		{
			"nested structure",
			`database:
  host: localhost
  port: 5432`,
			false,
			func(t *testing.T, cfg config.Config) {
				db := cfg.Any("database", nil)
				dbMap, ok := db.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "localhost", dbMap["host"])
				assert.Equal(t, 5432, dbMap["port"])
			},
		},
		{
			"list values",
			`tags:
  - alpha
  - beta
  - gamma`,
			false,
			func(t *testing.T, cfg config.Config) {
				tags := cfg.StringSlice("tags", nil)
				assert.Equal(t, []string{"alpha", "beta", "gamma"}, tags)
			},
		},
		{
			"empty yaml",
			``,
			false,
			func(t *testing.T, cfg config.Config) {
				assert.False(t, cfg.Has("anything"))
			},
		},
		{
			"invalid yaml",
			`invalid: yaml: content:`,
			true,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.FromYAML([]byte(tt.yaml))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

// TestFromJSON verifies JSON parsing.
func TestFromJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		check   func(*testing.T, config.Config)
	}{
		{
			"simple values",
			`{"name": "bob", "count": 100, "enabled": false}`,
			false,
			func(t *testing.T, cfg config.Config) {
				assert.Equal(t, "bob", cfg.String("name", ""))
				// JSON unmarshals numbers as float64
				assert.Equal(t, 100, cfg.Int("count", 0))
				assert.False(t, cfg.Bool("enabled", true))
			},
		},
		{
			"nested structure",
			`{"server": {"host": "127.0.0.1", "port": 8080}}`,
			false,
			func(t *testing.T, cfg config.Config) {
				server := cfg.Any("server", nil)
				serverMap, ok := server.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "127.0.0.1", serverMap["host"])
				// JSON numbers are float64
				assert.Equal(t, float64(8080), serverMap["port"])
			},
		},
		{
			"array values",
			`{"items": ["one", "two", "three"]}`,
			false,
			func(t *testing.T, cfg config.Config) {
				items := cfg.StringSlice("items", nil)
				assert.Equal(t, []string{"one", "two", "three"}, items)
			},
		},
		{
			"empty json",
			`{}`,
			false,
			func(t *testing.T, cfg config.Config) {
				assert.False(t, cfg.Has("anything"))
			},
		},
		{
			"invalid json",
			`{invalid json}`,
			true,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.FromJSON([]byte(tt.json))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

// TestFromFile verifies file loading with extension detection.
func TestFromFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create YAML file
	yamlPath := filepath.Join(tmpDir, "config.yaml")
	yamlContent := []byte(`name: fromyaml
value: 123`)
	require.NoError(t, os.WriteFile(yamlPath, yamlContent, 0o644))

	// Create YML file
	ymlPath := filepath.Join(tmpDir, "config.yml")
	ymlContent := []byte(`name: fromyml
value: 456`)
	require.NoError(t, os.WriteFile(ymlPath, ymlContent, 0o644))

	// Create JSON file
	jsonPath := filepath.Join(tmpDir, "config.json")
	jsonContent := []byte(`{"name": "fromjson", "value": 789}`)
	require.NoError(t, os.WriteFile(jsonPath, jsonContent, 0o644))

	// Create unsupported extension file
	txtPath := filepath.Join(tmpDir, "config.txt")
	require.NoError(t, os.WriteFile(txtPath, []byte("content"), 0o644))

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
		check   func(*testing.T, config.Config)
	}{
		{
			"yaml file",
			yamlPath,
			false,
			"",
			func(t *testing.T, cfg config.Config) {
				assert.Equal(t, "fromyaml", cfg.String("name", ""))
				assert.Equal(t, 123, cfg.Int("value", 0))
			},
		},
		{
			"yml file",
			ymlPath,
			false,
			"",
			func(t *testing.T, cfg config.Config) {
				assert.Equal(t, "fromyml", cfg.String("name", ""))
				assert.Equal(t, 456, cfg.Int("value", 0))
			},
		},
		{
			"json file",
			jsonPath,
			false,
			"",
			func(t *testing.T, cfg config.Config) {
				assert.Equal(t, "fromjson", cfg.String("name", ""))
				assert.Equal(t, 789, cfg.Int("value", 0))
			},
		},
		{
			"unsupported extension",
			txtPath,
			true,
			"unsupported config file extension",
			nil,
		},
		{
			"file not found",
			filepath.Join(tmpDir, "nonexistent.yaml"),
			true,
			"read config file",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.FromFile(tt.path)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

// TestFromFile_CaseInsensitiveExtension verifies extension matching is case-insensitive.
func TestFromFile_CaseInsensitiveExtension(t *testing.T) {
	tmpDir := t.TempDir()

	// Create uppercase YAML file
	yamlPath := filepath.Join(tmpDir, "config.YAML")
	yamlContent := []byte(`name: uppercase`)
	require.NoError(t, os.WriteFile(yamlPath, yamlContent, 0o644))

	// Create mixed case JSON file
	jsonPath := filepath.Join(tmpDir, "config.Json")
	jsonContent := []byte(`{"name": "mixedcase"}`)
	require.NoError(t, os.WriteFile(jsonPath, jsonContent, 0o644))

	cfg, err := config.FromFile(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "uppercase", cfg.String("name", ""))

	cfg, err = config.FromFile(jsonPath)
	require.NoError(t, err)
	assert.Equal(t, "mixedcase", cfg.String("name", ""))
}

// TestDuration_EdgeCases verifies edge cases for duration parsing.
func TestDuration_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		value      any
		defaultVal time.Duration
		want       time.Duration
	}{
		{"zero int", 0, time.Second, 0},
		{"zero float", 0.0, time.Second, 0},
		{"zero string", "0s", time.Second, 0},
		{"negative int", -5, time.Second, -5 * time.Second},
		{"negative string", "-5s", time.Second, -5 * time.Second},
		{"milliseconds string", "500ms", time.Second, 500 * time.Millisecond},
		{"microseconds string", "100us", time.Second, 100 * time.Microsecond},
		{"nanoseconds string", "1000ns", time.Second, 1000 * time.Nanosecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New(map[string]any{"d": tt.value})
			got := cfg.Duration("d", tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestInt_LargeNumbers verifies handling of large numbers.
func TestInt_LargeNumbers(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  int
	}{
		{"max int32", int(2147483647), 2147483647},
		{"large int64", int64(9223372036854775807), 9223372036854775807},
		{"large float64 whole", float64(1e10), 10000000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New(map[string]any{"n": tt.value})
			got := cfg.Int("n", 0)
			assert.Equal(t, tt.want, got)
		})
	}
}
