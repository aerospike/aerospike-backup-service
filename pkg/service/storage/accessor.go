package storage

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/backup-go"
)

// Accessor interface abstracts storage layer.
type Accessor interface {
	supports(storage model.Storage) bool
	createReader(
		ctx context.Context, storage model.Storage, path string, isFile bool, filter Validator, startScanFrom string,
	) (backup.StreamingReader, error)
	createWriter(ctx context.Context, storage model.Storage, path string, isFile, isRemoveFiles, withNested bool,
	) (backup.Writer, error)
}

var accessors []Accessor

// registerAccessor adds a new storage accessor.
func registerAccessor(accessor Accessor) {
	accessors = append(accessors, accessor)
}

// getAccessor returns the appropriate accessor for the given storage.
func getAccessor(storage model.Storage) Accessor {
	for _, accessor := range accessors {
		if accessor.supports(storage) {
			return accessor
		}
	}

	panic(fmt.Sprintf("unsupported storage type %T", storage))
}
