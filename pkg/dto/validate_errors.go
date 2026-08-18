package dto

import (
	"errors"
	"fmt"
	"strings"
)

var (
	errValidation        = errors.New("validation error")
	errRequiredEither    = fmt.Errorf("required fields %w", errValidation)
	errMutuallyExclusive = fmt.Errorf("mutually exclusive fields %w", errValidation)
	errEmpty             = fmt.Errorf("empty field %w", errValidation)
	errNotFound          = fmt.Errorf("not found %w", errValidation)
	errNonPositive       = fmt.Errorf("non-positive value %w", errValidation)
	errNegative          = fmt.Errorf("negative value %w", errValidation)
	errInvalidValue      = fmt.Errorf("invalid value %w", errValidation)
	errMissingDependency = fmt.Errorf("missing dependent field %w", errValidation)
	errDuplicate         = fmt.Errorf("duplicate value %w", errValidation)
	errSecretRefNoAgent  = fmt.Errorf("secret reference without agent %w", errValidation)
)

func errValidationRequiredEither(fields ...string) error {
	return fmt.Errorf("%w: must specify either of: %v", errRequiredEither, strings.Join(fields, ","))
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

func errValidationExceed(field string, value, maxAllowed any) error {
	return fmt.Errorf("%w: '%v' is not a valid %s. Should not exceed %v",
		errInvalidValue, value, field, maxAllowed)
}

func errValidationInvalidValue(field string, value, allowed any) error {
	return fmt.Errorf("%w: '%v' is not a valid %s. Allowed values: %v",
		errInvalidValue, value, field, allowed)
}

func errValidationRequires(setField, requiredField string) error {
	return fmt.Errorf("%w: %q requires %q to be set", errMissingDependency, setField, requiredField)
}

func errValidationDuplicate[T any](field string, value T) error {
	return fmt.Errorf("%w: %s contains duplicate value: %v", errDuplicate, field, value)
}

func errValidationSecretRefNoAgent(value secret) error {
	return fmt.Errorf("%w: %q requires secret agent configuration (secret-agent or secret-agent-name)",
		errSecretRefNoAgent, string(value))
}
