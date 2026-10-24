package collections

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
)

func TestPtr(t *testing.T) {
	var n int32
	np := ptr.Of(n)
	assert.Equal(t, n, *np)
}

func TestValueOrZero(t *testing.T) {
	i1 := 1
	assert.Equal(t, 1, ptr.ValueOrZero(&i1))
	var i2 *int
	assert.Equal(t, 0, ptr.ValueOrZero(i2))
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
			assert.Len(t, result, len(tt.expected))
			if len(tt.expected) > 0 {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFirstCommon(t *testing.T) {
	tests := map[string]struct {
		a, b   []string
		want   string
		wantOK bool
	}{
		"common element is found": {
			a:      []string{"ns1", "ns2"},
			b:      []string{"ns2", "ns3"},
			want:   "ns2",
			wantOK: true,
		},
		"disjoint slices have none": {
			a: []string{"ns1"},
			b: []string{"ns2"},
		},
		"empty b has none": {
			a: []string{"ns1"},
			b: nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, ok := FirstMatch(tt.a, tt.b)

			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUnique(t *testing.T) {
	tests := map[string]struct {
		input []string
		want  []string
	}{
		"empty slice": {
			input: nil,
			want:  nil,
		},
		"no duplicates": {
			input: []string{"ns1", "ns2", "ns3"},
			want:  []string{"ns1", "ns2", "ns3"},
		},
		"with duplicates": {
			input: []string{"ns1", "ns2", "ns1", "ns3", "ns2"},
			want:  []string{"ns1", "ns2", "ns3"},
		},
		"all duplicates": {
			input: []string{"ns1", "ns1", "ns1"},
			want:  []string{"ns1"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := Unique(tt.input)

			assert.Equal(t, tt.want, got)
		})
	}
}
