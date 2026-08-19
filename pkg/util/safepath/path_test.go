package safepath

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateClean(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "empty path", path: ""},
		{name: "clean relative path", path: "testdata/password.txt"},
		{name: "clean absolute path", path: "/etc/ssl/certs/ca.pem"},
		{name: "parent traversal", path: "certs/../../outside.pem", wantErr: true},
		{name: "leading parent traversal segment", path: "../etc/passwd", wantErr: true},
		{name: "dot prefix", path: "./certs/ca.pem", wantErr: true},
		{name: "redundant separators", path: "/etc//ssl/ca.pem", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateClean(tt.path)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
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
	defer root.Close()

	_, err = root.ReadFile("../secret.txt")
	require.Error(t, err)
}

func TestStat(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "cert.pem")
	require.NoError(t, os.WriteFile(filePath, []byte("cert"), 0600))

	info, err := Stat(filePath)
	require.NoError(t, err)
	require.False(t, info.IsDir())
}

func TestReadDir(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "a.pem"), []byte("a"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "b.pem"), []byte("b"), 0600))

	entries, err := ReadDir(tempDir)
	require.NoError(t, err)
	require.Len(t, entries, 2)
}
