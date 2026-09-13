// Package redact holds the Secret type: a string that redacts itself in logs, fmt output
// and API responses, and is read only through Reveal().
package redact

import (
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"strings"
)

const (
	// Placeholder is emitted for literal secret values in API responses and logs.
	Placeholder     = "[secret]"
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

	return Placeholder
}

// String implements fmt.Stringer for "%s" and "%v".
func (s Secret) String() string {
	return s.DisplayString()
}

// GoString redacts fmt "%#v" output used in debug prints and some test failure messages.
func (s Secret) GoString() string {
	if s == "" {
		return "redact.Secret(\"\")"
	}

	if s.isRef() {
		return fmt.Sprintf("redact.Secret(%q)", string(s))
	}

	return `redact.Secret("[secret]")`
}

// LogValue implements slog.LogValuer for direct slog.Any("password", secret) calls.
func (s Secret) LogValue() slog.Value {
	return slog.StringValue(s.DisplayString())
}

func (s Secret) IsRedacted() bool {
	return s == Placeholder
}

// Reveal returns the literal value. It is the only sanctioned way to read a secret; grep
// for it to find every place a credential leaves the type. Do not use a plain string
// conversion, which is invisible to such a search.
func (s Secret) Reveal() string {
	return string(s)
}

// Hash returns a hash of the literal value. Use it wherever a configuration hash must
// change when the secret changes: String() and %v render the redaction placeholder, so
// hashing the formatted value would make all literal secrets collide.
func (s Secret) Hash() uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))

	return h.Sum64()
}
