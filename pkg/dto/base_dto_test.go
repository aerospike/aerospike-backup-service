package dto

import (
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertModelsToDTO(t *testing.T) {
	t.Parallel()

	input := []int{1, 2, 3}
	result := ConvertModelsToDTO(input, func(m *int) string {
		return strings.Repeat("x", *m)
	})

	require.Len(t, result, 3)
	assert.Equal(t, []string{"x", "xx", "xxx"}, result)
}

func TestConvertModelMapToDTO(t *testing.T) {
	t.Parallel()

	input := map[string]*int{"a": ptrInt(1), "b": ptrInt(2)}
	result := ConvertModelMapToDTO(input, func(m *int) *string {
		s := strings.Repeat("y", *m)
		return &s
	})

	require.Len(t, result, 2)
	assert.Equal(t, "y", *result["a"])
	assert.Equal(t, "yy", *result["b"])
}

func TestConvertStorageMapToDTO(t *testing.T) {
	t.Parallel()

	modelMap := map[string]model.Storage{
		"local1": &model.LocalStorage{Path: "/tmp/backups"},
	}
	config := &model.BackupConfig{Storage: modelMap}

	result := ConvertStorageMapToDTO(modelMap, config)
	require.Len(t, result, 1)
	require.NotNil(t, result["local1"])
	require.NotNil(t, result["local1"].LocalStorage)
	assert.Equal(t, "/tmp/backups", result["local1"].LocalStorage.Path)
}

func ptrInt(v int) *int { return &v }
