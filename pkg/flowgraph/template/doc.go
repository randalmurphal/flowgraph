/*
Package template provides variable expansion for strings.

# Overview

template expands ${var} and $var patterns in strings using provided
variable maps. It's designed for configuration templates, URL construction,
and dynamic string building in workflow contexts.

# Basic Usage

Expand variables using the package-level function:

	result := template.Expand("Hello ${name}", map[string]any{"name": "World"})
	// result: "Hello World"

Both brace and dollar-sign patterns are supported:

	vars := map[string]any{"host": "api.example.com", "port": 8080}
	url := template.Expand("https://${host}:$port/api", vars)
	// url: "https://api.example.com:8080/api"

# Variable Patterns

Two patterns are supported:

  - ${var} - Brace style, recommended for clarity
  - $var - Dollar style, simpler but requires word boundaries

The dollar style uses word boundary detection to avoid partial matches.
For example, $port won't match inside $portNumber.

# Missing Variables

By default, missing variables are kept as-is:

	result := template.Expand("Hello ${missing}", nil)
	// result: "Hello ${missing}"

Configure behavior with options:

	exp := template.NewExpander(template.WithMissingAction(template.MissingEmpty))
	result, _ := exp.Expand("Hello ${missing}", nil)
	// result: "Hello "

	exp = template.NewExpander(template.WithMissingAction(template.MissingError))
	_, err := exp.Expand("Hello ${missing}", nil)
	// err: "undefined variable: missing"

# Batch Expansion

Expand multiple strings or maps efficiently:

	vars := map[string]any{"env": "prod"}

	// Expand slice of strings
	urls := template.ExpandAll([]string{
	    "https://${env}.api.com",
	    "https://${env}.db.com",
	}, vars)

	// Expand all string values in a map recursively
	config := template.ExpandMap(map[string]any{
	    "url": "https://${env}.api.com",
	    "nested": map[string]any{
	        "endpoint": "/api/${env}/v1",
	    },
	}, vars)

# Custom Expander

Create a custom expander for advanced scenarios:

	exp := template.NewExpander(
	    template.WithMissingAction(template.MissingError),
	    template.WithBraceStyle(true),
	    template.WithDollarStyle(false), // Disable $var pattern
	)

	result, err := exp.Expand("${greeting} ${name}", map[string]any{
	    "greeting": "Hello",
	    "name": "World",
	})

# Thread Safety

Expander is safe for concurrent use after construction.
Package-level functions use a shared default expander.
*/
package template
