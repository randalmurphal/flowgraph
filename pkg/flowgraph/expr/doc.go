/*
Package expr provides expression evaluation for flowgraph conditions.

# Overview

expr implements a simple expression language for evaluating conditions in
workflow graphs. It supports comparison operators, logical operators, and
variable resolution from a context map.

# Expression Syntax

	<expr> := <comparison>
	        | <expr> 'and' <expr>
	        | <expr> 'or' <expr>
	        | 'not' <expr>
	        | '!' <expr>
	        | <value>

	<comparison> := <value> <op> <value>
	<op> := '==' | '!=' | '<' | '>' | '<=' | '>=' | 'contains'
	<value> := 'string' | "string" | number | true | false | null | identifier

# Operators

Comparison operators:

	==         Equal (string comparison)
	!=         Not equal (string comparison)
	<          Less than (numeric comparison)
	>          Greater than (numeric comparison)
	<=         Less than or equal (numeric comparison)
	>=         Greater than or equal (numeric comparison)
	contains   String contains substring

Logical operators:

	and        Logical AND
	or         Logical OR
	not        Logical NOT (prefix)
	!          Logical NOT (prefix)

# Value Types

Values can be:

  - Quoted strings: 'hello' or "hello"
  - Numbers: 42, 3.14, -1
  - Booleans: true, false
  - Null: null, nil
  - Variables: referenced by name from the vars map

# Examples

Simple comparisons:

	status == 'active'          // String equality
	count > 10                  // Numeric comparison
	name != ''                  // Not empty string

Logical operators:

	status == 'ready' and count > 0
	enabled or override
	not disabled
	!cancelled

Variable resolution:

	vars := map[string]any{"status": "active", "count": 5}
	result, _ := expr.Eval("status == 'active'", vars)  // true
	result, _ := expr.Eval("count > 10", vars)          // false

Contains operator:

	message contains 'error'    // true if message contains "error"

# Custom Operators

Register custom binary operators:

	e := expr.New(
	    expr.WithCustomOperator("matches", func(left, right any) bool {
	        pattern := fmt.Sprintf("%v", right)
	        value := fmt.Sprintf("%v", left)
	        matched, _ := regexp.MatchString(pattern, value)
	        return matched
	    }),
	)
	result, _ := e.Evaluate("name matches '^test.*'", vars)

# Truthiness

Single values are evaluated for truthiness:

  - nil/null: false
  - bool: the boolean value
  - string: false if empty, true otherwise
  - numbers (int, int64, float64): false if zero, true otherwise
  - other types: true
*/
package expr
