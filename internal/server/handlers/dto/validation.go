package dto

import "fmt"

func emptyFieldValidationError(field string) error {
	return fmt.Errorf("empty %s is not allowed", field)
}

func notFoundValidationError(field string, value string) error {
	return fmt.Errorf("%s '%s' not found", field, value)
}
