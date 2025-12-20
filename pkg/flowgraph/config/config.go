package config

import (
	"time"
)

// Config wraps a map[string]any for type-safe value extraction.
// All accessor methods return default values if the key is missing
// or the value cannot be converted to the requested type.
type Config struct {
	data map[string]any
}

// New creates a Config from the given map.
// If data is nil, an empty Config is returned.
func New(data map[string]any) Config {
	if data == nil {
		data = make(map[string]any)
	}
	return Config{data: data}
}

// String returns the string value for key, or defaultVal if missing or not a string.
func (c Config) String(key, defaultVal string) string {
	v, ok := c.data[key]
	if !ok {
		return defaultVal
	}
	if s, ok := v.(string); ok {
		return s
	}
	return defaultVal
}

// Duration returns the duration value for key, or defaultVal if missing or invalid.
//
// Accepts:
//   - string: parsed with time.ParseDuration
//   - int: interpreted as seconds
//   - int64: interpreted as seconds
//   - float64: interpreted as seconds
//   - time.Duration: used directly
func (c Config) Duration(key string, defaultVal time.Duration) time.Duration {
	v, ok := c.data[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case string:
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	case float64:
		return time.Duration(val * float64(time.Second))
	case int:
		return time.Duration(val) * time.Second
	case int64:
		return time.Duration(val) * time.Second
	case time.Duration:
		return val
	}
	return defaultVal
}

// Bool returns the boolean value for key, or defaultVal if missing or not a bool.
func (c Config) Bool(key string, defaultVal bool) bool {
	v, ok := c.data[key]
	if !ok {
		return defaultVal
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return defaultVal
}

// Int returns the integer value for key, or defaultVal if missing or not convertible.
//
// Accepts:
//   - int: used directly
//   - int64: converted to int
//   - float64: converted to int (truncated, only if no fractional part)
func (c Config) Int(key string, defaultVal int) int {
	v, ok := c.data[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		// Only convert if there's no fractional part
		if val == float64(int(val)) {
			return int(val)
		}
	}
	return defaultVal
}

// Float returns the float64 value for key, or defaultVal if missing or not convertible.
//
// Accepts:
//   - float64: used directly
//   - int: converted to float64
//   - int64: converted to float64
func (c Config) Float(key string, defaultVal float64) float64 {
	v, ok := c.data[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	}
	return defaultVal
}

// StringSlice returns the string slice for key, or defaultVal if missing or not convertible.
//
// Accepts:
//   - []string: used directly
//   - []any: each element converted to string if possible
func (c Config) StringSlice(key string, defaultVal []string) []string {
	v, ok := c.data[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			} else {
				// If any element isn't a string, return default
				return defaultVal
			}
		}
		return result
	}
	return defaultVal
}

// Any returns the raw value for key, or defaultVal if missing.
func (c Config) Any(key string, defaultVal any) any {
	v, ok := c.data[key]
	if !ok {
		return defaultVal
	}
	return v
}

// Has returns true if the key exists in the config.
func (c Config) Has(key string) bool {
	_, ok := c.data[key]
	return ok
}

// Raw returns the underlying map.
// The returned map should not be modified.
func (c Config) Raw() map[string]any {
	return c.data
}
