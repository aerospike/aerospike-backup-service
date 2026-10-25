package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsertKeyAt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		index    int
		key      string
		wantKeys []string
	}{
		{name: "new key", index: 1, key: "x", wantKeys: []string{"a", "x", "b", "c"}},
		{name: "new key at end", index: 3, key: "x", wantKeys: []string{"a", "b", "c", "x"}},
		{name: "existing key moved forward", index: 2, key: "a", wantKeys: []string{"b", "a", "c"}},
		{name: "existing key moved back", index: 0, key: "c", wantKeys: []string{"c", "a", "b"}},
		{name: "existing key in place", index: 1, key: "b", wantKeys: []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			obj := &jsonObject{
				keys:   []string{"a", "b", "c"},
				values: map[string]any{"a": "old", "b": "old", "c": "old"},
			}

			obj.insertKeyAt(tt.index, tt.key, "new")

			assert.Equal(t, tt.wantKeys, obj.keys)
			assert.Equal(t, "new", obj.values[tt.key])
		})
	}
}
