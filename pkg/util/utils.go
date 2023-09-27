package util

// Ptr returns a pointer to the given object.
func Ptr[T any](obj T) *T {
	return &obj
}
