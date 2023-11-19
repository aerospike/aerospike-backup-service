//nolint:typecheck
package util

// Ptr returns a pointer to the given object.
func Ptr[T any](obj T) *T {
	return &obj
}

func Find[T any](collection map[string]T, f func(T) bool) (T, bool) {
	for _, item := range collection {
		if f(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}
