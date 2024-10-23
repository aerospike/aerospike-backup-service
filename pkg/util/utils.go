package util

import (
	"fmt"
	"net/url"
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

// ValueOrZero dereferences a pointer and returns the value.
// Zero value is returned if the pointer is nil.
func ValueOrZero[T any](p *T) T {
	if p != nil {
		return *p
	}
	var zero T
	return zero
}

// TryAndRecover executes the given function `f` and recovers from any panics
// that occur.
func TryAndRecover(f func() string) (output string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from: %v", r)
			output = ""
		}
	}()
	return f(), err
}

// ParseS3Path parses an S3 path and returns the bucket and path components.
// The path is trimmed of the leading slash (/).
// Amazon S3 require paths to be without slashes.
func ParseS3Path(s string) (bucket string, path string, err error) {
	parsed, err := url.Parse(s)
	if err != nil {
		return "", "", err
	}

	return parsed.Host, strings.TrimPrefix(parsed.Path, "/"), nil
}

// MissingElements returns all elements in `subset` that are not present in `superset`.
func MissingElements(subset, superset []string) []string {
	// Create a map to store elements of `superset` for quick lookup.
	elementSet := make(map[string]struct{})
	for _, element := range superset {
		elementSet[element] = struct{}{}
	}

	// Collect elements of `subset` that do not exist in the map.
	var missing []string
	for _, element := range subset {
		if _, found := elementSet[element]; !found {
			missing = append(missing, element)
		}
	}

	return missing
}
