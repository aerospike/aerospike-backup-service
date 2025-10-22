package service

import (
	"maps"
	"strconv"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestExtractTimestampFromPath(t *testing.T) {
	service := NewPathService(nil)

	t.Run("valid backup path", func(t *testing.T) {
		path := "routine/backup/1234567890123/"
		timestamp := service.ExtractTimestampFromPath(path)
		assert.Equal(t, "1234567890123", timestamp)
	})

	t.Run("valid incremental path", func(t *testing.T) {
		path := "routine/incremental/1234567890123/"
		timestamp := service.ExtractTimestampFromPath(path)
		assert.Equal(t, "1234567890123", timestamp)
	})

	t.Run("path with human date", func(t *testing.T) {
		now := time.Now()

		for format := range maps.Keys(model.TimestampFormatPresets) {
			service := NewPathService(&format)
			formatTimestamp := service.GetBackupPath("routine", jobTypeFull, "ns1", now)
			timestamp := service.ExtractTimestampFromPath(formatTimestamp)
			assert.Equal(t, strconv.FormatInt(now.UnixMilli(), 10), timestamp)
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		path := "routine/invalid/1234567890123/"
		timestamp := service.ExtractTimestampFromPath(path)
		assert.Empty(t, timestamp)
	})

	t.Run("path with short timestamp", func(t *testing.T) {
		path := "routine/backup/12345/"
		timestamp := service.ExtractTimestampFromPath(path)
		assert.Empty(t, timestamp)
	})
}

func TestPathService_GetConfigurationFilePath(t *testing.T) {
	pathService := NewPathService(nil)
	path := pathService.GetConfigurationFilePath("routine", time.Now(), 1)
	assert.Contains(t, path, "routine/backup/")
	assert.Contains(t, path, "/configuration/aerospike_1.conf")
}
