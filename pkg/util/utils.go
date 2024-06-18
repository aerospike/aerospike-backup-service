//nolint:typecheck
package util

import (
	"fmt"
	"net/url"
	"os"
	"strings"
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
	if p != nil {
		return *p
	}
	var zero T
	return zero
}

// TryAndRecover executes the given function `f` and recovers from any panics that occur.
func TryAndRecover(f func() string) (output string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("an error occurred: %v", r)
			output = ""
		}
	}()
	return f(), err
}

func ParseS3Path(s string) (bucket string, path string, err error) {
	parsed, err := url.Parse(s)
	if err != nil {
		return "", "", err
	}

	return parsed.Host, strings.TrimPrefix(parsed.Path, "/"), nil
}
