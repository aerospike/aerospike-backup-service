package collections

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
