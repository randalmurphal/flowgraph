/*
Package config provides type-safe configuration extraction from map[string]any.

# Overview

config wraps a map[string]any and provides typed accessor methods that handle
missing keys and type mismatches gracefully by returning default values.
This is useful for extracting configuration values from YAML/JSON structures
without verbose type assertions and nil checks.

# Basic Usage

Create a Config from any map and extract values with defaults:

	cfg := config.New(map[string]any{
	    "timeout": "30s",
	    "retries": 3,
	    "enabled": true,
	})

	timeout := cfg.Duration("timeout", 10*time.Second) // 30s
	retries := cfg.Int("retries", 5)                   // 3
	enabled := cfg.Bool("enabled", false)              // true
	missing := cfg.String("missing", "default")        // "default"

# Type Coercion

Duration handles multiple input types:
  - string: parsed with time.ParseDuration ("30s", "1h30m")
  - int/float64: interpreted as seconds
  - time.Duration: used directly

Numeric types handle reasonable conversions:
  - int from float64 (truncated)
  - float64 from int

All methods return the default value if:
  - The key is missing
  - The value cannot be converted to the requested type
  - The conversion would lose precision (e.g., float to int with fraction)

# File Loading

Load configuration from YAML or JSON files:

	cfg, err := config.FromFile("config.yaml")
	if err != nil {
	    log.Fatal(err)
	}

	// Or load from bytes
	cfg, err = config.FromYAML(yamlBytes)
	cfg, err = config.FromJSON(jsonBytes)

# Thread Safety

Config is safe for concurrent read access. The underlying map is not
modified after creation. However, if the original map is modified
externally, behavior is undefined.
*/
package config
