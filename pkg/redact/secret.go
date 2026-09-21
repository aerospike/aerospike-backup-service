// Package redact holds the Secret type: a string that redacts itself in logs, fmt output
// and API responses, and is read only through Reveal().
package redact

import (
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

// Redactable is implemented by credential-bearing values that know how to render themselves
// safely. The reflective walks in pkg/dto/decoder key on this interface rather than on Secret
// itself, so a second secret-bearing type is redacted, and merge-preserved, without touching
// them: the walk stores whatever Redacted returns where the value was found, and never looks
// at how the value is represented.
type Redactable interface {
	// IsRedacted reports whether the value is the redaction placeholder itself.
	IsRedacted() bool
	// Redacted returns the value as it may be handed to anyone, with the credential replaced.
	// It must have the same type as its receiver, so it fits the field, map entry or slice
	// element the receiver came from; the receiver itself is untouched.
	Redacted() Redactable
}

var _ Redactable = Secret("")

// IsMalformedRef reports whether the value looks like a Secret Agent reference but is not
// well-formed. DTO-layer validation (pkg/dto) uses this to reject it; Secret itself only
// needs it to decide how to redact and display the value.
func (s Secret) IsMalformedRef() bool {
	return strings.HasPrefix(string(s), secretRefPrefix) && !s.IsRef()
}

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

	return Placeholder
}

// Redacted returns the value as it is shown: a literal becomes the Placeholder, a well-formed
// secret agent reference and an empty value stay as they are.
func (s Secret) Redacted() Redactable {
	return Secret(s.DisplayString())
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

	if s.IsRef() {
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
