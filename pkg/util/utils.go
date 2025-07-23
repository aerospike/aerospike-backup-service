package util

import (
	"fmt"
	"runtime/debug"
	"slices"
	"time"
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

// ValueOrZero dereferences a pointer and returns the value.
// Zero value is returned if the pointer is nil.
func ValueOrZero[T any](p *T) T {
	if p != nil {
		return *p
	}
	var zero T
	return zero
}

// TryAndRecover executes a function and recovers from any panic.
func TryAndRecover[T any](f func() T) (output T, err error) {
	defer func() {
		if r := recover(); r != nil {
			stackTrace := string(debug.Stack())
			err = fmt.Errorf("recovered from panic: %v\nStack trace:\n%s", r, stackTrace)
		}
	}()
	return f(), nil
}

// TryAndRecoverError executes a function returning (T, error) and recovers from any panic.
func TryAndRecoverError[T any](f func() (T, error)) (output T, err error) {
	defer func() {
		if r := recover(); r != nil {
			stackTrace := string(debug.Stack())
			err = fmt.Errorf("recovered from panic: %v\nStack trace:\n%s", r, stackTrace)
		}
	}()
	return f()
}

// MissingElements returns all elements in `subset` that are not present in `superset`.
func MissingElements(subset, superset []string) []string {
	var missing []string
	for _, element := range subset {
		if !slices.Contains(superset, element) {
			missing = append(missing, element)
		}
	}

	return missing
}

func MeasureDuration(f func() error) (time.Duration, error) {
	startTime := time.Now()
	err := f()
	return time.Since(startTime), err
}
