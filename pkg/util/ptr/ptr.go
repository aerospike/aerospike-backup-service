package ptr

// Of returns a pointer to the given object.
func Of[T any](obj T) *T {
	return &obj
}

// ValueOrZero dereferences a pointer and returns the value.
// Zero value is returned if the pointer is nil.
func ValueOrZero[T any](p *T) T {
	if p != nil {
		return *p
	}
	var zero T
	return zero
}

// StringOrNil returns nil when s is empty, otherwise a pointer to s.
func StringOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return Of(s)
}
