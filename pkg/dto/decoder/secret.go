package decoder

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

const (
	// redactedSecret is the placeholder emitted for literal secret values in API responses and logs.
	redactedSecret  = "[secret]"
	secretRefPrefix = "secrets:"
)

// Secret marks a string field as sensitive for redaction during API responses and logging.
type Secret string

// ErrSecretValidation is returned when a secret value fails validation.
var ErrSecretValidation = errors.New("secret validation failed")

// Validate checks secret agent references.
func (s Secret) Validate(withAgent bool) error {
	if s == "" {
		return nil
	}

	if s.isMalformedRef() {
		return fmt.Errorf("%w: %q must be in the form secrets:<resource>:<key>", ErrSecretValidation, string(s))
	}

	if s.isRef() && !withAgent {
		return fmt.Errorf("%w: %q requires secret agent configuration (secret-agent or secret-agent-name)",
			ErrSecretValidation, string(s))
	}

	return nil
}

// isMalformedRef reports whether the value looks like a secret agent reference but is not well-formed.
func (s Secret) isMalformedRef() bool {
	return strings.HasPrefix(string(s), secretRefPrefix) && !s.isRef()
}

// isRef reports whether the value is a well-formed Secret Agent reference.
func (s Secret) isRef() bool {
	asString := string(s)
	if asString == "" {
		return false
	}

	if !strings.HasPrefix(asString, secretRefPrefix) {
		return false
	}

	return strings.Count(asString, ":") == 2
}

// DisplayString returns a safe string for logs and errors: secret agent references are shown
// as-is; literal secrets are redacted.
func (s Secret) DisplayString() string {
	if s == "" {
		return ""
	}

	if s.isRef() {
		return string(s)
	}

	return redactedSecret
}

// String implements fmt.Stringer for "%s" and "%v".
func (s Secret) String() string {
	return s.DisplayString()
}

// GoString redacts fmt "%#v" output used in debug prints and some test failure messages.
func (s Secret) GoString() string {
	if s == "" {
		return "decoder.Secret(\"\")"
	}

	if s.isRef() {
		return fmt.Sprintf("decoder.Secret(%q)", string(s))
	}

	return `decoder.Secret("[secret]")`
}

// LogValue implements slog.LogValuer for direct slog.Any("password", secret) calls.
func (s Secret) LogValue() slog.Value {
	return slog.StringValue(s.DisplayString())
}
