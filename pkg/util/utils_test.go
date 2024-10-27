package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPtr(t *testing.T) {
	var n int32
	np := Ptr(n)
	if n != *np {
		t.Error("Expected to be equal")
	}
}

func TestFind(t *testing.T) {
	elements := map[string]int{"a": 1}
	found := Find(elements, func(i int) bool { return i == 1 })
	if found == nil {
		t.Error("Expected to be found")
	}
	found = Find(elements, func(i int) bool { return i == 2 })
	if found != nil {
		t.Error("Expected not to be found")
	}
}

func TestValueOrZero(t *testing.T) {
	i1 := 1
	if ValueOrZero(&i1) != 1 {
		t.Error("Expected 1")
	}
	var i2 *int
	if ValueOrZero(i2) != 0 {
		t.Error("Expected 0")
	}
}

func TestMissingElements(t *testing.T) {
	tests := []struct {
		name     string
		subset   []string
		superset []string
		expected []string
	}{
		{
			name:     "empty subset and superset",
			subset:   []string{},
			superset: []string{},
			expected: []string{},
		},
		{
			name:     "empty subset",
			subset:   []string{},
			superset: []string{"a", "b", "c"},
			expected: []string{},
		},
		{
			name:     "empty superset",
			subset:   []string{"a", "b", "c"},
			superset: []string{},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "no missing elements",
			subset:   []string{"a", "b"},
			superset: []string{"a", "b", "c"},
			expected: []string{},
		},
		{
			name:     "some missing elements",
			subset:   []string{"a", "b", "d", "e"},
			superset: []string{"a", "b", "c"},
			expected: []string{"d", "e"},
		},
		{
			name:     "all elements missing",
			subset:   []string{"x", "y", "z"},
			superset: []string{"a", "b", "c"},
			expected: []string{"x", "y", "z"},
		},
		{
			name:     "duplicate elements in subset",
			subset:   []string{"a", "b", "b", "c", "c"},
			superset: []string{"a"},
			expected: []string{"b", "b", "c", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MissingElements(tt.subset, tt.superset)
			assert.Equal(t, len(tt.expected), len(result))
			if len(tt.expected) > 0 {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
