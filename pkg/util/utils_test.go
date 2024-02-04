package util

import (
	"testing"
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
