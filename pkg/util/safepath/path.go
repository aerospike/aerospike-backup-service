package safepath

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ValidateClean reports whether path is safe for config use.
// Empty paths are allowed and return nil.
// Paths must be in canonical form (no ./ prefix, redundant separators, etc.)
// and must not contain ".." elements.
func ValidateClean(path string) error {
	if path == "" {
		return nil
	}

	cleaned := filepath.Clean(path)
	if cleaned != path {
		return fmt.Errorf("path %q is not clean (normalizes to %q)", path, cleaned)
	}

	if slices.Contains(strings.Split(path, string(filepath.Separator)), "..") {
		return fmt.Errorf("path %q must not contain traversal", path)
	}

	return nil
}

// EnsureFileExists checks that a validated file path exists using os.Root.
// path must have passed ValidateClean.
func EnsureFileExists(path string) error {
	if path == "" {
		return nil
	}

	root, name, err := openRootForFile(path)
	if err != nil {
		return err
	}
	defer root.Close()

	info, err := root.Stat(name)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path %q is a directory, not a file", path)
	}

	return nil
}

// ReadFile reads the full contents of a validated file path using os.Root.
func ReadFile(path string) ([]byte, error) {
	root, name, err := openRootForFile(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	info, err := root.Stat(name)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path %q is a directory, not a file", path)
	}

	return root.ReadFile(name)
}

// ReadDir lists entries in a validated directory path using os.Root.
func ReadDir(path string) ([]fs.DirEntry, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	return fs.ReadDir(root.FS(), ".")
}

// openRootForFile splits a validated file path into its directory and base name.
func openRootForFile(path string) (*os.Root, string, error) {
	dir, name := filepath.Split(path)
	if name == "" {
		return nil, "", fmt.Errorf("path %q is not a file path", path)
	}

	if dir == "" {
		dir = "."
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, "", err
	}

	return root, name, nil
}
