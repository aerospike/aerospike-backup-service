package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestOperations(t *testing.T) (*operations, *model.LocalStorage) {
	t.Helper()

	return newLocalOperations(t), &model.LocalStorage{Path: t.TempDir()}
}

func newLocalOperations(t *testing.T) *operations {
	t.Helper()

	ops, ok := NewOperations(NewLocalStorageAccessor()).(*operations)
	require.True(t, ok)

	return ops
}

func TestNewOperations(t *testing.T) {
	t.Parallel()

	ops := newLocalOperations(t)
	require.NotNil(t, ops)
	assert.Len(t, ops.accessors, 1)
}

func TestOperations_WriteDataFile_ReadFile(t *testing.T) {
	t.Parallel()

	ops, s := newTestOperations(t)
	ctx := t.Context()
	content := []byte("hello data")

	require.NoError(t, ops.WriteDataFile(ctx, s, "file.asb", content))

	got, err := ops.ReadFile(ctx, s, "file.asb")
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestOperations_WriteMetadataFile_ReadFile(t *testing.T) {
	t.Parallel()

	ops, s := newTestOperations(t)
	ctx := t.Context()
	content := []byte("metadata content")

	require.NoError(t, ops.WriteMetadataFile(ctx, s, "metadata.yaml", content))

	got, err := ops.ReadFile(ctx, s, "metadata.yaml")
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestOperations_ReadFile_NotFound(t *testing.T) {
	t.Parallel()

	ops, s := newTestOperations(t)

	_, err := ops.ReadFile(t.Context(), s, "does-not-exist.asb")
	require.Error(t, err)
}

func TestOperations_ReadFiles(t *testing.T) {
	t.Parallel()

	ops, s := newTestOperations(t)
	ctx := t.Context()

	files := map[string]string{
		"sub/aaa.keep": "content-aaa",
		"sub/bbb.skip": "content-bbb",
		"sub/ccc.keep": "content-ccc",
	}
	for name, content := range files {
		require.NoError(t, ops.WriteDataFile(ctx, s, name, []byte(content)))
	}

	buffers, err := ops.ReadFiles(ctx, s, "sub", ".keep")
	require.NoError(t, err)
	require.Len(t, buffers, 2)

	contents := make([]string, len(buffers))
	for i, b := range buffers {
		contents[i] = b.String()
	}
	assert.ElementsMatch(t, []string{"content-aaa", "content-ccc"}, contents)
}

func TestOperations_ReadFiles_CreateReaderError(t *testing.T) {
	t.Parallel()

	ops, s := newTestOperations(t)

	_, err := ops.ReadFiles(t.Context(), s, "missing-dir", "")
	require.Error(t, err)
}

func TestOperations_ReadFileNames(t *testing.T) {
	t.Parallel()

	ops, s := newTestOperations(t)
	ctx := t.Context()

	files := map[string]string{
		"sub/aaa.keep": "content-aaa",
		"sub/bbb.skip": "content-bbb",
		"sub/ccc.keep": "content-ccc",
	}
	for name, content := range files {
		require.NoError(t, ops.WriteDataFile(ctx, s, name, []byte(content)))
	}

	names, err := ops.ReadFileNames(ctx, s, "sub", ".keep", nil)
	require.NoError(t, err)
	require.Len(t, names, 2)

	expectedDir := filepath.Join(s.GetPath(), "sub")
	assert.ElementsMatch(t, []string{
		filepath.Join(expectedDir, "aaa.keep"),
		filepath.Join(expectedDir, "ccc.keep"),
	}, names)
}

func TestOperations_ReadFileNames_WithFromTime(t *testing.T) {
	t.Parallel()

	ops, s := newTestOperations(t)
	ctx := t.Context()

	require.NoError(t, ops.WriteDataFile(ctx, s, "sub/aaa.keep", []byte("content-aaa")))

	fromTime := time.Now()
	names, err := ops.ReadFileNames(ctx, s, "sub", ".keep", &fromTime)
	require.NoError(t, err)
	assert.Len(t, names, 1)
}

func TestOperations_DeleteFolder(t *testing.T) {
	t.Parallel()

	ops, s := newTestOperations(t)
	ctx := t.Context()

	require.NoError(t, ops.WriteDataFile(ctx, s, "sub/aaa.asb", []byte("content-aaa")))
	require.NoError(t, ops.WriteDataFile(ctx, s, "sub/bbb.asb", []byte("content-bbb")))

	require.NoError(t, ops.DeleteFolder(ctx, s, "sub"))

	names, err := ops.ReadFileNames(ctx, s, "sub", "", nil)
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestOperations_GetAccessor_Unsupported(t *testing.T) {
	t.Parallel()

	ops := newLocalOperations(t)

	_, err := ops.getAccessor(&model.S3Storage{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported storage type")
}

func TestOperations_ReadFile_UnsupportedStorage(t *testing.T) {
	t.Parallel()

	ops := newLocalOperations(t)

	_, err := ops.ReadFile(t.Context(), &model.S3Storage{}, "file.asb")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported storage type")
}

func TestOperations_UnsupportedStorage_ErrorPaths(t *testing.T) {
	t.Parallel()

	ops := newLocalOperations(t)
	unsupported := &model.S3Storage{}
	ctx := t.Context()

	t.Run("WriteDataFile", func(t *testing.T) {
		t.Parallel()
		err := ops.WriteDataFile(ctx, unsupported, "file.asb", []byte("x"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create writer")
	})

	t.Run("WriteMetadataFile", func(t *testing.T) {
		t.Parallel()
		err := ops.WriteMetadataFile(ctx, unsupported, "meta.yaml", []byte("x"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create writer")
	})

	t.Run("ReadFiles", func(t *testing.T) {
		t.Parallel()
		_, err := ops.ReadFiles(ctx, unsupported, "sub", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create reader")
	})

	t.Run("ReadFileNames", func(t *testing.T) {
		t.Parallel()
		_, err := ops.ReadFileNames(ctx, unsupported, "sub", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create reader")
	})

	t.Run("DeleteFolder", func(t *testing.T) {
		t.Parallel()
		err := ops.DeleteFolder(ctx, unsupported, "sub")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported storage type")
	})
}
