package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/io/storage/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalStorageAccessor(t *testing.T) {
	t.Parallel()

	require.NotNil(t, NewLocalStorageAccessor())
}

func TestLocalStorageAccessor_Supports(t *testing.T) {
	t.Parallel()

	a := NewLocalStorageAccessor()

	assert.True(t, a.supports(&model.LocalStorage{Path: "/tmp"}))
	assert.False(t, a.supports(&model.S3Storage{}))
}

func TestLocalStorageAccessor_CreateReader(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.asb"), []byte("hello"), 0o600))

	a := NewLocalStorageAccessor()
	reader, err := a.createReader(t.Context(), &model.LocalStorage{Path: dir}, options.WithDir(dir))
	require.NoError(t, err)
	require.NotNil(t, reader)
}

func TestLocalStorageAccessor_CreateWriter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := NewLocalStorageAccessor()

	writer, err := a.createWriter(t.Context(), &model.LocalStorage{Path: dir}, options.WithDir(dir))
	require.NoError(t, err)
	require.NotNil(t, writer)
}

func TestLocalStorageAccessor_CreateWriter_WithMinPartSize(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := NewLocalStorageAccessor()
	minPartSize := 1024

	writer, err := a.createWriter(
		t.Context(),
		&model.LocalStorage{Path: dir, MinPartSize: &minPartSize},
		options.WithDir(dir),
	)
	require.NoError(t, err)
	require.NotNil(t, writer)
}

func TestLocalStorageAccessor_Probe(t *testing.T) {
	t.Parallel()

	existing := t.TempDir()

	regularFile := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(regularFile, []byte("x"), 0o600))

	readOnly := filepath.Join(t.TempDir(), "read-only")
	require.NoError(t, os.Mkdir(readOnly, 0o500))

	tests := []struct {
		name        string
		path        string
		expectError string
	}{
		{
			name: "existing writable directory",
			path: existing,
		},
		{
			// The local writer creates missing directories, so a path that is not there
			// yet is fine as long as an existing ancestor can be written to.
			name: "directory the writer will create",
			path: filepath.Join(existing, "backups", "routine"),
		},
		{
			name:        "path is a regular file",
			path:        regularFile,
			expectError: "is not a directory",
		},
		{
			name:        "directory is not writable",
			path:        readOnly,
			expectError: "write permission check failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.path == readOnly && os.Geteuid() == 0 {
				t.Skip("root ignores directory permissions")
			}

			err := NewLocalStorageAccessor().probe(t.Context(), &model.LocalStorage{Path: tt.path})

			if tt.expectError == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.expectError)
		})
	}
}

// The probe must not leave its test file behind in the backup destination.
func TestLocalStorageAccessor_Probe_LeavesNoFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, NewLocalStorageAccessor().probe(t.Context(), &model.LocalStorage{Path: dir}))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
