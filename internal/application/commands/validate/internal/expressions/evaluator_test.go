package expressions

import "testing"

func TestIsTruthyJMESPathSemantics(t *testing.T) {
	testCases := []struct {
		name     string
		value    any
		expected bool
	}{
		{name: "false", value: false, expected: false},
		{name: "null", value: nil, expected: false},
		{name: "empty string", value: "", expected: false},
		{name: "empty array", value: []any{}, expected: false},
		{name: "empty object", value: map[string]any{}, expected: false},
		{name: "true", value: true, expected: true},
		{name: "number", value: 0, expected: true},
		{name: "non-empty string", value: "x", expected: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := IsTruthy(tc.value)
			if actual != tc.expected {
				t.Fatalf("unexpected truthiness for %#v: expected %v, got %v", tc.value, tc.expected, actual)
			}
		})
	}
}
