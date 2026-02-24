package collections

import (
	"maps"
	"slices"
)

// Keys returns all keys of the map as a slice.
func Keys[V any](m map[string][]V) []string {
	return slices.Collect(maps.Keys(m))
}

// Flatten returns a single slice containing all values from all slices in the map.
func Flatten[V any](m map[string][]V) []V {
	total := 0
	for _, slice := range m {
		total += len(slice)
	}

	out := make([]V, 0, total)

	for _, slice := range m {
		out = append(out, slice...)
	}

	return out
}
