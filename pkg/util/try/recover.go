package try

import (
	"fmt"
	"runtime/debug"
)

// Recover executes a function and recovers from any panic.
func Recover[T any](f func() T) (output T, err error) {
	defer func() {
		if r := recover(); r != nil {
			stackTrace := string(debug.Stack())
			err = fmt.Errorf("recovered from panic: %v\nStack trace:\n%s", r, stackTrace)
		}
	}()
	return f(), nil
}

// RecoverError executes a function returning (T, error) and recovers from any panic.
func RecoverError[T any](f func() (T, error)) (output T, err error) {
	defer func() {
		if r := recover(); r != nil {
			stackTrace := string(debug.Stack())
			err = fmt.Errorf("recovered from panic: %v\nStack trace:\n%s", r, stackTrace)
		}
	}()
	return f()
}
