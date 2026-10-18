package dto

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

func validateUniqueNonEmpty(field string, items []string) error {
	if err := validateUnique(field, items); err != nil {
		return err
	}

	for i, item := range items {
		if item == "" {
			return errValidationEmptyField(fmt.Sprintf("%s[%d]", field, i))
		}
	}

	return nil
}

func validateUnique[T comparable](field string, items []T) error {
	if duplicates := collections.CheckDuplicates(items); len(duplicates) > 0 {
		return errValidationDuplicate(field, duplicates)
	}

	return nil
}
