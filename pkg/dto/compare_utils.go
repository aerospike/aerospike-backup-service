package dto

import (
	"errors"
	"fmt"
)

func comparePointers[T comparable](fieldName string, oldValue, newValue *T) error {
	if oldValue == nil && newValue == nil {
		return nil
	}

	if oldValue == nil {
		return fmt.Errorf("%s added", fieldName)
	}

	if newValue == nil {
		return fmt.Errorf("%s removed", fieldName)
	}

	return compareValues(fieldName, *oldValue, *newValue)
}

func compareValues[T comparable](fieldName string, oldValue, newValue T) error {
	if oldValue != newValue {
		return fmt.Errorf("%s changed: %v -> %v", fieldName, oldValue, newValue)
	}

	return nil
}

func compareSlices[T comparable](fieldName string, oldSlice []T, newSlice []T) error {
	if len(oldSlice) != len(newSlice) {
		return fmt.Errorf("%s length changed: %d -> %d", fieldName, len(oldSlice), len(newSlice))
	}

	var err error
	for i, v := range oldSlice {
		if v != newSlice[i] {
			err = errors.Join(err, fmt.Errorf("%s[%d] changed: %v -> %v", fieldName, i, v, newSlice[i]))
		}
	}

	return err
}
