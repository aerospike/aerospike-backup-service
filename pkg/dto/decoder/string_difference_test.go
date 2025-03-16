package decoder

import (
	"testing"
)

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1, s2   string
		expected int
	}{
		{"", "", 0},              // Identical empty strings
		{"abc", "abc", 0},        // Identical strings
		{"abc", "abd", 1},        // One substitution
		{"hello", "helo", 1},     // One character removed
		{"ab", "abc", 1},         // One insertion
		{"port", "prot", 2},      // Two substitutions
		{"flaw", "lawn", 2},      // Standard case
		{"kitten", "sitting", 3}, // Classic case, exceeds maxStringsDifference
		{"abcdef", "ghijkl", 3},  // No common characters, exceeds maxStringsDifference
	}

	for _, test := range tests {
		got := levenshteinDistance(test.s1, test.s2)
		if got != test.expected {
			t.Errorf("distance(%q, %q) = %d; want %d", test.s1, test.s2, got, test.expected)
		}
	}
}
