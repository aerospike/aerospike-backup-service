package safepath

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadFileRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "certs"), 0755))

	_, err := ReadFile(filepath.Join(dir, "certs"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "directory")
}

func TestReadFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "secret.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("secret"), 0600))

	data, err := ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, []byte("secret"), data)
}

func TestReadFileRejectsRootEscape(t *testing.T) {
	rootDir := t.TempDir()
	innerDir := filepath.Join(rootDir, "inner")
	require.NoError(t, os.Mkdir(innerDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "secret.txt"), []byte("secret"), 0600))

	root, err := os.OpenRoot(innerDir)
	require.NoError(t, err)
	defer func() { _ = root.Close() }()

	_, err = root.ReadFile("../secret.txt")
	require.Error(t, err)
}

func TestReadDir(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "a.pem"), []byte("a"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "b.pem"), []byte("b"), 0600))

	entries, err := ReadDir(tempDir)
	require.NoError(t, err)
	require.Len(t, entries, 2)
}
