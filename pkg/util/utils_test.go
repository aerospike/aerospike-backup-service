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
