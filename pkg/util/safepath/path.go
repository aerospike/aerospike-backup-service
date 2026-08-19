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

// Stat returns file metadata for a validated clean path using os.Root.
func Stat(path string) (fs.FileInfo, error) {
	root, name, err := openRootForFile(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	return root.Stat(name)
}

// ReadFile reads the full contents of a file at a validated clean path using os.Root.
func ReadFile(path string) ([]byte, error) {
	root, name, err := openRootForFile(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	return root.ReadFile(name)
}

// ReadDir lists entries in a validated clean directory path using os.Root.
func ReadDir(path string) ([]fs.DirEntry, error) {
	if err := ValidateClean(path); err != nil {
		return nil, err
	}

	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	return fs.ReadDir(root.FS(), ".")
}

// OpenRoot opens an os.Root for the directory part of a file path.
func OpenRootForFile(path string) (*os.Root, string, error) {
	return openRootForFile(path)
}

func openRootForFile(path string) (*os.Root, string, error) {
	dir, name := filepath.Split(path)
	if name == "" {
		return nil, "", fmt.Errorf("path %q is not a file path", path)
	}

	if dir == "" {
		dir = "."
	} else {
		dir = filepath.Clean(dir)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, "", err
	}

	return root, name, nil
}
