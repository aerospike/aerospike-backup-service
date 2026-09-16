package dto

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Configuration names end up in the storage layout: a routine's backups live under
// <storage root>/<routine>/<type>/<timestamp>/data/<namespace>, and retention removes
// folders by the same path. A name that is not a single path segment would place all of
// that outside the configured root, so names are validated here rather than where the
// paths are built.

const (
	// maxNamespaceNameLength is the Aerospike limit on a namespace name.
	// See https://aerospike.com/docs/database/reference/limitations/#namespace
	maxNamespaceNameLength = 31
	// reservedNamespaceName is the one name the Aerospike server keeps for itself.
	reservedNamespaceName = "null"
	// namespaceNameChars is the full set of characters an Aerospike namespace name may use.
	namespaceNameChars = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"0123456789_-$"
)

// NamespaceName represents a validated Aerospike namespace name in the DTO layer.
// A namespace is not only a name in the cluster: it is also the folder a routine's data for
// that namespace is written to, so an unchecked name reaches the storage layout.
type NamespaceName string

// Validate checks the name against the Aerospike naming rules: at most 31 bytes of Latin
// letters, digits, '_', '-' and '$', and not the reserved name "null". Those rules are
// stricter than a single path segment has to be, so they cover the storage layout as well.
// See https://aerospike.com/docs/database/reference/limitations/#namespace
//
// Validate reports only the reason. Call sites must wrap with errValidationInvalidName, which
// adds the field name and the offending value, and classifies the error (errEmpty for a
// missing name, errInvalidValue for an unusable one).
func (n NamespaceName) Validate() error {
	if n == "" {
		return errEmpty
	}

	if len(n) > maxNamespaceNameLength {
		return fmt.Errorf("must not exceed %d bytes", maxNamespaceNameLength)
	}

	if n == reservedNamespaceName {
		return fmt.Errorf("%q is reserved by the Aerospike server", reservedNamespaceName)
	}

	// TrimLeft drops the leading run of allowed characters, so the remainder, if any,
	// starts with the first character that is not allowed.
	if rest := strings.TrimLeft(string(n), namespaceNameChars); rest != "" {
		r, _ := utf8.DecodeRuneInString(rest)

		return fmt.Errorf(
			"must contain only Latin letters, digits, \"_\", \"-\" and \"$\", found %q", r)
	}

	return nil
}

// newNamespaceNames adopts the model's plain strings as namespace names.
func newNamespaceNames(values []string) []NamespaceName {
	names := make([]NamespaceName, len(values))
	for i, value := range values {
		names[i] = NamespaceName(value)
	}

	return names
}

// namespaceStrings returns the names as the plain strings the model carries.
func namespaceStrings(names []NamespaceName) []string {
	values := make([]string, len(names))
	for i, name := range names {
		values[i] = string(name)
	}

	return values
}

// validateEntityName checks that the key of a routine, storage, cluster, policy or secret
// agent is a single path segment.
func validateEntityName(field, name string) error {
	return errValidationInvalidName(field, name, checkPathSegment(name))
}

// checkPathSegment reports why name cannot be used as one element of a path.
func checkPathSegment(name string) error {
	switch {
	case name == "":
		return errEmpty
	case strings.ContainsRune(name, 0):
		return errors.New("must not contain a NUL byte")
	case name == "." || name == "..":
		return errors.New("must not be \".\" or \"..\" (path traversal not allowed)")
	case strings.ContainsAny(name, `/\`):
		return errors.New(`must not contain a path separator ("/" or "\")`)
	case strings.HasPrefix(name, "~"):
		return errors.New(`must not start with "~": home-directory expansion is not supported`)
	}

	return nil
}
