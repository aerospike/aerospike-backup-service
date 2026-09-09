package dto

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// Path represents a validated file system path in DTO layer.
// It ensures paths are in canonical form and safe for use.
type Path string

// Validate checks if the path is safe and in canonical form.
//
// By default validation is strictest: the path must be non-empty and
// storage-relative. Callers relax it per field:
//   - ValidationAllowEmpty: an empty path is a valid "not configured" value.
//   - ValidationAllowAbsolutePath: the path may be absolute, for local filesystem
//     paths such as certificates, key files and log files.
//
// Paths must never contain ".." elements and must be in canonical form
// (no ./ prefix, redundant separators, or trailing slashes).
//
// Call sites must wrap with errValidationInvalidPath, which adds both and classifies
// the error (errEmpty for a missing path, errInvalidPath for an unusable one).
func (p Path) Validate(opts ValidationOptions) error {
	path := string(p)
	if path == "" {
		if opts.Has(ValidationAllowEmpty) {
			return nil
		}

		return fmt.Errorf("%w: must not be empty", errEmpty)
	}

	// Check for path traversal first (most critical security issue)
	if slices.Contains(strings.Split(path, string(filepath.Separator)), "..") {
		return errors.New("must not contain '..' (path traversal not allowed)")
	}

	// Check for non-canonical form (includes trailing slashes, ./ prefix, redundant separators, etc.)
	cleaned := filepath.Clean(path)
	if cleaned != path {
		// Provide specific hint for trailing slashes
		if strings.HasSuffix(path, "/") {
			return fmt.Errorf("must not end with '/' (use %q instead)", cleaned)
		}
		return fmt.Errorf("must be in canonical form (expected %q)", cleaned)
	}

	if !opts.Has(ValidationAllowAbsolutePath) && !filepath.IsLocal(path) {
		return errors.New("must be relative to the storage root")
	}

	return nil
}
