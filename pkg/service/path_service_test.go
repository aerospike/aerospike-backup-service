package service

import (
	"maps"
	"strconv"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
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
			formatTimestamp := service.GetBackupPath("routine", model.BackupTypeFull, "ns1", now)
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

func TestPathService_GetBackupAttemptPath(t *testing.T) {
	pathService := NewPathService(nil)
	ts := time.UnixMilli(1700000000000)

	tests := []struct {
		name    string
		attempt int
		want    string
	}{
		{name: "first attempt uses the namespace folder", attempt: 1, want: "r/backup/1700000000000/data/ns_1"},
		{name: "retry uses a sibling folder", attempt: 2, want: "r/backup/1700000000000/data/ns_1.2"},
		{name: "later retry", attempt: 5, want: "r/backup/1700000000000/data/ns_1.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pathService.GetBackupAttemptPath("r", model.BackupTypeFull, "ns_1", ts, tt.attempt))
		})
	}

	// Retention removes a backup by the timestamp folder above its key, which holds for an
	// attempt folder too, so leftovers of failed attempts go with the run.
	attemptKey := pathService.GetBackupAttemptPath("r", model.BackupTypeFull, "ns_1", ts, 2)
	assert.Equal(t, pathService.GetTimestampPath("r", ts, model.BackupTypeFull), extractBackupDirFromKey(attemptKey))
	assert.Equal(t, "1700000000000", pathService.ExtractTimestampFromPath(attemptKey+"/"+metadataFile))
}

func TestExtractBackupDirFromKey(t *testing.T) {
	routineName := "test-routine"
	namespace := "test-ns"
	backupType := model.BackupTypeFull
	now := time.Now()

	for _, format := range []*model.TimestampFormat{nil, ptr.Of(model.TimestampFormatEU)} {
		pathService := NewPathService(format)

		t.Run(string(ptr.ValueOrZero(format)), func(t *testing.T) {
			// construct path
			backupPath := pathService.GetBackupPath(routineName, backupType, namespace, now)
			// deconstruct path
			backupDir := extractBackupDirFromKey(backupPath)
			// get expected path
			expectedPath := pathService.GetTimestampPath(routineName, now, backupType)

			assert.Equal(t, expectedPath, backupDir)
		})
	}
}

func TestFormatTimestampSuffixIsUTC(t *testing.T) {
	format := model.TimestampFormatEU
	pathService := NewPathService(&format)
	utcTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	offsetTime := utcTime.In(time.FixedZone("UTC-5", -5*3600))

	got := pathService.GetTimestampPath("routine", offsetTime, model.BackupTypeFull)
	utcSuffix := utcTime.Format(model.TimestampFormatPresets[format])
	localSuffix := offsetTime.Format(model.TimestampFormatPresets[format])

	assert.Contains(t, got, utcSuffix)
	assert.NotEqual(t, utcSuffix, localSuffix)
	assert.NotContains(t, got, localSuffix)
}
