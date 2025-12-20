package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FromFile loads configuration from a file, auto-detecting format by extension.
// Supported extensions: .yaml, .yml, .json
func FromFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return FromYAML(data)
	case ".json":
		return FromJSON(data)
	default:
		return Config{}, fmt.Errorf("unsupported config file extension: %s", ext)
	}
}

// FromYAML parses YAML data into a Config.
func FromYAML(data []byte) (Config, error) {
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Config{}, fmt.Errorf("parse yaml: %w", err)
	}
	return New(m), nil
}

// FromJSON parses JSON data into a Config.
func FromJSON(data []byte) (Config, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return Config{}, fmt.Errorf("parse json: %w", err)
	}
	return New(m), nil
}
