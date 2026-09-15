package dto

import (
	"errors"
	"fmt"
	"strings"
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
)

// validateEntityName checks that the key of a routine, storage, cluster, policy or secret
// agent is a single path segment.
func validateEntityName(field, name string) error {
	if name == "" {
		return errValidationEmptyField(field)
	}

	if err := checkPathSegment(name); err != nil {
		return errValidationInvalidName(field, name, err)
	}

	return nil
}

// validateNamespaceName checks a namespace name against the Aerospike naming rules, which are
// stricter than a path segment needs to be and therefore cover that too.
// See https://aerospike.com/docs/database/reference/limitations/#namespace
func validateNamespaceName(field, namespace string) error {
	if namespace == "" {
		return errValidationEmptyField(field)
	}

	if err := checkNamespaceName(namespace); err != nil {
		return errValidationInvalidName(field, namespace, err)
	}

	return nil
}

// checkPathSegment reports why name cannot be used as one element of a path.
func checkPathSegment(name string) error {
	switch {
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

// checkNamespaceName reports why namespace is not a valid Aerospike namespace name.
func checkNamespaceName(namespace string) error {
	if len(namespace) > maxNamespaceNameLength {
		return fmt.Errorf("must not exceed %d bytes", maxNamespaceNameLength)
	}

	if namespace == reservedNamespaceName {
		return fmt.Errorf("%q is reserved by the Aerospike server", reservedNamespaceName)
	}

	for _, r := range namespace {
		if !isNamespaceNameRune(r) {
			return fmt.Errorf(
				"must contain only Latin letters, digits, \"_\", \"-\" and \"$\", found %q", r)
		}
	}

	return nil
}

func isNamespaceNameRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == '_', r == '-', r == '$':
		return true
	default:
		return false
	}
}
