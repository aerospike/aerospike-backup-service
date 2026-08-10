package dto

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

const secretRedactedPlaceholder = "[REDACTED]"

// SecretString holds a sensitive string value that is redacted when logged.
type SecretString struct {
	value string
}

// NewSecretString creates a SecretString from a plain string.
func NewSecretString(s string) SecretString {
	return SecretString{value: s}
}

// SecretStringPtr returns a pointer to a SecretString with the given value.
func SecretStringPtr(s string) *SecretString {
	return &SecretString{value: s}
}

// Value returns the underlying secret value.
func (s SecretString) Value() string {
	return s.value
}

// String redacts the secret for logging.
func (s SecretString) String() string {
	if s.value == "" {
		return ""
	}

	return secretRedactedPlaceholder
}

// IsSet reports whether the secret has a non-empty value.
func (s SecretString) IsSet() bool {
	return s.value != ""
}

// SecretStringValue returns the underlying value from a pointer, or "" if nil.
func SecretStringValue(s *SecretString) string {
	if s == nil {
		return ""
	}

	return s.value
}

// SecretStringIsSet reports whether the pointer is non-nil and has a non-empty value.
func SecretStringIsSet(s *SecretString) bool {
	return s != nil && s.value != ""
}

func secretStringFromModelPtr(s *string) *SecretString {
	if s == nil || *s == "" {
		return nil
	}

	return &SecretString{value: *s}
}

func secretStringToModelPtr(s *SecretString) *string {
	if s == nil {
		return nil
	}

	value := s.value
	return &value
}

func (s SecretString) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.value)
}

func (s *SecretString) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &s.value)
}

func (s SecretString) MarshalYAML() (any, error) {
	return s.value, nil
}

func (s *SecretString) UnmarshalYAML(value *yaml.Node) error {
	return value.Decode(&s.value)
}

// Format implements fmt.Formatter so %v and %+v redact secrets.
func (s SecretString) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v', 's':
		if s.value == "" {
			_, _ = fmt.Fprintf(f, "%s", "")
			return
		}

		_, _ = fmt.Fprintf(f, "%s", secretRedactedPlaceholder)
	default:
		_, _ = fmt.Fprintf(f, "%%!%c(SecretString=%s)", verb, s.String())
	}
}
