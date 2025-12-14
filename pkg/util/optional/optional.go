package optional

// Optional is a container for a value that might be missing.
// It solves the problem of distinguishing between "0" and "nil".
type Optional[T any] struct {
	Value   T
	Present bool
}

// Of creates a new Optional with a value.
func Of[T any](val T) Optional[T] {
	return Optional[T]{
		Value:   val,
		Present: true,
	}
}

// FromPtr converts a pointer to an Optional.
// nil -> Empty, non-nil -> Of(*ptr).
func FromPtr[T any](ptr *T) Optional[T] {
	if ptr == nil {
		return Empty[T]()
	}

	return Optional[T]{Value: *ptr, Present: true}
}

// ToPtr converts an Optional to a pointer.
// Empty -> nil, Present -> &Value.
func (o Optional[T]) ToPtr() *T {
	if !o.Present {
		return nil
	}

	return &o.Value
}

// Empty returns a missing value.
func Empty[T any]() Optional[T] {
	return Optional[T]{
		Present: false,
	}
}

// Get returns the value and a boolean indicating if it was present.
func (o Optional[T]) Get() (T, bool) {
	return o.Value, o.Present
}

// Or returns the current Optional if it is present.
// Otherwise, it returns the fallback Optional.
func (o Optional[T]) Or(fallback Optional[T]) Optional[T] {
	if o.Present {
		return o
	}

	return fallback
}

// OrElse returns the value if present, otherwise the default fallback.
func (o Optional[T]) OrElse(fallback T) T {
	if o.Present {
		return o.Value
	}

	return fallback
}
