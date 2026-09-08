package dto

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// Path represents a validated file system path in DTO layer.
// It ensures paths are in canonical form and safe for use.
// Empty paths are allowed.
type Path string

// Validate checks if the path is safe and in canonical form.
// Empty paths are allowed and return nil.
// Paths must not contain ".." elements and must be in canonical form
// (no ./ prefix, redundant separators, or trailing slashes).
func (p Path) Validate() error {
	path := string(p)
	if path == "" {
		return nil
	}

	// Check for path traversal first (most critical security issue)
	if slices.Contains(strings.Split(path, string(filepath.Separator)), "..") {
		return fmt.Errorf("path %q must not contain '..' (path traversal not allowed)", path)
	}

	// Check for non-canonical form (includes trailing slashes, ./ prefix, redundant separators, etc.)
	cleaned := filepath.Clean(path)
	if cleaned != path {
		// Provide specific hint for trailing slashes
		if strings.HasSuffix(path, "/") {
			return fmt.Errorf("path %q must not end with '/' (use %q instead)", path, cleaned)
		}
		return fmt.Errorf("path must be in canonical form (got %q, expected %q)", path, cleaned)
	}

	return nil
}

// ValidateRelative validates storage-relative paths such as object-storage
// prefixes and backup-data-path. Empty paths are allowed.
func (p Path) ValidateRelative() error {
	if p == "" {
		return nil
	}
	if err := p.Validate(); err != nil {
		return err
	}
	if !filepath.IsLocal(string(p)) {
		return fmt.Errorf("path %q must be relative", p)
	}

	return nil
}
