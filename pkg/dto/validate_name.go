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

// The Aerospike limits on names.
// See https://aerospike.com/docs/database/reference/limitations/
const (
	maxNamespaceNameLength = 31
	maxSetNameLength       = 63
	maxBinNameLength       = 15
	// reservedName is the one name the Aerospike server keeps for itself.
	reservedName = "null"
	// aerospikeNameChars is the full set of characters an Aerospike name may use.
	aerospikeNameChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-$"
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
	return checkAerospikeName(string(n), maxNamespaceNameLength)
}

// validateAerospikeNames checks a list of set or bin names for duplicates and each name against
// the Aerospike naming rules. The rules apply in both backup modes: their character set also
// keeps out the separators of the info commands that server-side backup sends the names in.
func validateAerospikeNames(field string, names []string, maxLength int) error {
	if err := validateUnique(field, names); err != nil {
		return err
	}

	for i, name := range names {
		err := errValidationInvalidName(fmt.Sprintf("%s[%d]", field, i), name, checkAerospikeName(name, maxLength))
		if err != nil {
			return err
		}
	}

	return nil
}

// checkAerospikeName reports why name breaks the Aerospike naming rules: at most maxLength bytes
// of Latin letters, digits, '_', '-' and '$', and not the reserved name "null".
func checkAerospikeName(name string, maxLength int) error {
	if name == "" {
		return errEmpty
	}

	if len(name) > maxLength {
		return fmt.Errorf("must not exceed %d bytes", maxLength)
	}

	if name == reservedName {
		return fmt.Errorf("%q is reserved by the Aerospike server", reservedName)
	}

	// TrimLeft drops the leading run of allowed characters, so the remainder, if any,
	// starts with the first character that is not allowed.
	if rest := strings.TrimLeft(name, aerospikeNameChars); rest != "" {
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

// validateRoutineName checks that a routine's name is a single path segment that an Aerospike
// info command can carry: server-side backup sends the routine's folder in its commands.
func validateRoutineName(name string) error {
	if err := validateEntityName("routine name", name); err != nil {
		return err
	}

	return validateInfoValue("routine name", name)
}

// checkPathSegment reports why name cannot be used as one element of a path.
func checkPathSegment(name string) error {
	switch {
	case name == "":
		return errEmpty
	case strings.TrimSpace(name) == "":
		return errors.New("must not be blank")
	case strings.TrimSpace(name) != name:
		// Surrounding whitespace survives into the folder name, where "daily " and "daily" are
		// two routines that list identically and cannot be told apart by an operator.
		return errors.New("must not start or end with whitespace")
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
