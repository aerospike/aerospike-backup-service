//nolint:typecheck
package util

import (
	"github.com/aerospike/backup/pkg/model"
)

// Ptr returns a pointer to the given object.
func Ptr[T any](obj T) *T {
	return &obj
}

func GetByName[T model.WithName](collection []T, name *string) (int, T) {
	if name != nil {
		for i, item := range collection {
			if item.GetName() != nil && *item.GetName() == *name {
				return i, item
			}
		}
	}
	var zero T
	return -1, zero
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
