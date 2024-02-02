//nolint:typecheck
package util

import (
	"os"
)

// Ptr returns a pointer to the given object.
func Ptr[T any](obj T) *T {
	return &obj
}

// Find returns the key of first element from the given map that satisfies
// the predicate f. Nil if not found.
func Find[T any](items map[string]T, f func(T) bool) *string {
	for key, item := range items {
		if f(item) {
			return &key
		}
	}
	return nil
}

func isRunningInDockerContainer() bool {
	_, found := os.LookupEnv("DOCKER_CONTAINER")
	return found
}

// ValueOrZero dereferences a pointer and returns the value.
// Zero value is returned if the pointer is nil.
func ValueOrZero[T any](p *T) T {
	var zero T
	if p != nil {
		return *p
	}
	return zero
}
