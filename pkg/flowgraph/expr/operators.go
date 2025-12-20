package expr

import (
	"fmt"
	"strings"
)

// Compare compares two values using the specified operator.
// Returns an error for unknown operators.
func Compare(left, right any, op string) (bool, error) {
	switch op {
	case "==":
		return compareEquals(left, right), nil
	case "!=":
		return compareNotEquals(left, right), nil
	case "<":
		return compareLT(left, right), nil
	case ">":
		return compareGT(left, right), nil
	case "<=":
		return compareLTE(left, right), nil
	case ">=":
		return compareGTE(left, right), nil
	case "contains":
		return compareContains(left, right), nil
	default:
		return false, fmt.Errorf("unknown operator: %s", op)
	}
}

// compareEquals compares if left equals right using string comparison.
func compareEquals(left, right any) bool {
	return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right)
}

// compareNotEquals compares if left does not equal right using string comparison.
func compareNotEquals(left, right any) bool {
	return fmt.Sprintf("%v", left) != fmt.Sprintf("%v", right)
}

// compareLT compares if left < right using numeric comparison.
func compareLT(left, right any) bool {
	l, r := ToFloat64(left), ToFloat64(right)
	return l < r
}

// compareGT compares if left > right using numeric comparison.
func compareGT(left, right any) bool {
	l, r := ToFloat64(left), ToFloat64(right)
	return l > r
}

// compareLTE compares if left <= right using numeric comparison.
func compareLTE(left, right any) bool {
	l, r := ToFloat64(left), ToFloat64(right)
	return l <= r
}

// compareGTE compares if left >= right using numeric comparison.
func compareGTE(left, right any) bool {
	l, r := ToFloat64(left), ToFloat64(right)
	return l >= r
}

// compareContains checks if left contains right as a substring.
func compareContains(left, right any) bool {
	return strings.Contains(fmt.Sprintf("%v", left), fmt.Sprintf("%v", right))
}
