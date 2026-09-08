package safepath

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ReadFile reads the full contents of a validated file path using os.Root.
func ReadFile(path string) ([]byte, error) {
	root, name, err := openRootForFile(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()

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
	defer func() { _ = root.Close() }()

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
