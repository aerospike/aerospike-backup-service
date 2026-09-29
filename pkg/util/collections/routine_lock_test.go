package collections

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLockMap_Get(t *testing.T) {
	var lm LockMap

	muA1 := lm.Get("a")
	muA2 := lm.Get("a")
	muB := lm.Get("b")

	assert.Same(t, muA1, muA2)
	assert.NotSame(t, muA1, muB)
}
