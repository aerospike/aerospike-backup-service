package dto

import (
	"fmt"
)

var (
	errValidation        = fmt.Errorf("validation error")
	errRequiredEither    = fmt.Errorf("required fields %w", errValidation)
	errMutuallyExclusive = fmt.Errorf("mutually exclusive fields %w", errValidation)
	errEmpty             = fmt.Errorf("empty field %w", errValidation)
	errNotFound          = fmt.Errorf("not found %w", errValidation)
	errNonPositive       = fmt.Errorf("non-positive value %w", errValidation)
	errNegative          = fmt.Errorf("negative value %w", errValidation)
	errInvalidValue      = fmt.Errorf("invalid value %w", errValidation)
	errMissingDependency = fmt.Errorf("missing dependent field %w", errValidation)
)

func errValidationRequiredEither(field1, field2 string) error {
	return fmt.Errorf("%w: must specify either %q or %q", errRequiredEither, field1, field2)
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

func errValidationNonPositive[T ~int | ~int8 | ~int16 | ~int32 | ~int64](field string, value T) error {
	return fmt.Errorf("%w: %q %d invalid, should be positive number", errNonPositive, field, value)
}

func errValidationNegative[T ~int | ~int8 | ~int16 | ~int32 | ~int64](field string, value T) error {
	return fmt.Errorf("%w: %q %d invalid, should not be negative number", errNegative, field, value)
}

func errValidationInvalidValue(field, value any, allowed any) error {
	return fmt.Errorf("%w: '%v' is not a valid %s. Allowed values: %v",
		errInvalidValue, value, field, allowed)
}

func errValidationMissingDependency(field1, field2 string) error {
	return fmt.Errorf("%w: %q requires %q to be set", errMissingDependency, field1, field2)
}
