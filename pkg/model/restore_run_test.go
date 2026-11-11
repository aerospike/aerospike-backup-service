package model

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRestoreCounters_Plus goes over the fields by reflection, so that a counter added to the
// struct and left out of Plus fails the test.
func TestRestoreCounters_Plus(t *testing.T) {
	t.Parallel()

	var a, b, want RestoreCounters
	for i := range reflect.TypeFor[RestoreCounters]().NumField() {
		reflect.ValueOf(&a).Elem().Field(i).SetUint(uint64(i + 1))
		reflect.ValueOf(&b).Elem().Field(i).SetUint(uint64(100 * (i + 1)))
		reflect.ValueOf(&want).Elem().Field(i).SetUint(uint64(101 * (i + 1)))
	}

	assert.Equal(t, want, a.Plus(b))
}
