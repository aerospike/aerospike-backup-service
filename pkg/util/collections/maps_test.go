package collections

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeys(t *testing.T) {
	assert.Empty(t, Keys[int](nil))
	assert.Empty(t, Keys(map[string][]int{}))

	m := map[string][]int{
		"b": {2},
		"a": {1},
	}
	keys := Keys(m)
	slices.Sort(keys)
	assert.Equal(t, []string{"a", "b"}, keys)
}

func TestFlatten(t *testing.T) {
	assert.Empty(t, Flatten[int](nil))
	assert.Empty(t, Flatten(map[string][]int{}))

	m := map[string][]int{
		"a": {1, 2},
		"b": {3},
		"c": {},
	}
	assert.ElementsMatch(t, []int{1, 2, 3}, Flatten(m))
}
