package decoder

import (
	"fmt"
	"log/slog"
	"strings"
)

const (
	// RedactedSecret is the placeholder emitted for literal secret values in API responses and logs.
	RedactedSecret  = "[secret]"
	secretRefPrefix = "secrets:"
)

// Secret marks a string field as sensitive for redaction during API responses and logging.
type Secret string

// IsRef reports whether the value is a well-formed Secret Agent reference.
func (s Secret) IsRef() bool {
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

	if s.IsRef() {
		return string(s)
	}

	return RedactedSecret
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

	if s.IsRef() {
		return fmt.Sprintf("decoder.Secret(%q)", string(s))
	}

	return `decoder.Secret("[secret]")`
}

// LogValue implements slog.LogValuer for direct slog.Any("password", secret) calls.
func (s Secret) LogValue() slog.Value {
	return slog.StringValue(s.DisplayString())
}
