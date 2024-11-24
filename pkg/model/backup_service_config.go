package model

import (
	"errors"
	"fmt"
)

// BackupServiceConfig represents the backup service configuration properties.
// @Description BackupServiceConfig represents the backup service configuration properties.
type BackupServiceConfig struct {
	// HTTPServer is the backup service HTTP server configuration.
	HTTPServer *HTTPServerConfig
	// Logger is the backup service logger configuration.
	Logger *LoggerConfig
}

// Compare BackupServiceConfig with another and return detailed errors.
func (c *BackupServiceConfig) Compare(other BackupServiceConfig) error {
	var err error

	if e := c.HTTPServer.Compare(other.HTTPServer); e != nil {
		err = errors.Join(err, fmt.Errorf("HTTPServer changes: %w", e))
	}

	if e := c.Logger.Compare(other.Logger); e != nil {
		err = errors.Join(err, fmt.Errorf("logger changes: %w", e))
	}

	return err
}

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
