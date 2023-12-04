//nolint:typecheck
package util

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
