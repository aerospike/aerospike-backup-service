package dto

import "fmt"

var (
	errValidation        = fmt.Errorf("validation error")
	errRequired          = fmt.Errorf("required fields %w", errValidation)
	errMutuallyExclusive = fmt.Errorf("mutually exclusive fields %w", errValidation)
	errEmpty             = fmt.Errorf("empty field %w", errValidation)
	errNotFound          = fmt.Errorf("not found %w", errValidation)
)

func errValidationRequiredField(field1, field2 string) error {
	return fmt.Errorf("%w: must specify either %q or %q", errRequired, field1, field2)
}

func errValidationMutuallyExclusive(field1, field2 string) error {
	return fmt.Errorf("%w: cannot specify both %q and %q", errMutuallyExclusive, field1, field2)
}

func errValidationEmptyField(field string) error {
	return fmt.Errorf("%w: %q required", errEmpty, field)
}

func errValidationNotFound(field, value string) error {
	return fmt.Errorf("%w: %s %q", errNotFound, field, value)
}
