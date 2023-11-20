//nolint:typecheck
package util

// Ptr returns a pointer to the given object.
func Ptr[T any](obj T) *T {
	return &obj
}

// Find returns the first element from the given map that satisfies
// the predicate f.
func Find[T any](items map[string]T, f func(T) bool) (T, bool) {
	for _, item := range items {
		if f(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}
