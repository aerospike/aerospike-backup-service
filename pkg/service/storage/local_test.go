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
