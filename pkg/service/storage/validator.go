package storage

import (
	"fmt"
	"strings"

	"github.com/aerospike/backup-go/io/storage"
)

// Validator is used inside of backup.StreamingReader to filter files.
type Validator interface {
	// Run validates a file path and returns an error if it should be skipped.
	Run(fileName string) error
}

type nameValidator struct {
	filter string
}

func newNameValidator(s string) *nameValidator {
	if s != "" {
		return &nameValidator{s}
	}

	return nil
}

func (n *nameValidator) Run(path string) error {
	if n == nil || len(n.filter) == 0 {
		return nil
	}

	if strings.HasSuffix(path, n.filter) {
		return nil
	}

	return fmt.Errorf("skipped by filter '%s'", n.filter)
}

// ErrEmptyStorage indicates that there are no files to restore in the source directory.
var ErrEmptyStorage = storage.ErrEmptyStorage
