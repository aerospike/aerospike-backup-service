package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutineFilter_String(t *testing.T) {
	routine := &model.BackupRoutine{
		Name:    "routine-1",
		Storage: &model.LocalStorage{Path: "/data"},
	}

	str := NewFullBackupFilter(routine).Last().String()

	assert.Contains(t, str, "routine: routine-1")
	assert.Contains(t, str, "storage: LocalStorage(Path: /data)")
	assert.Contains(t, str, "type: full")
	assert.Contains(t, str, "last: true")
	assert.Contains(t, str, "timebounds: NA")
}

func TestPathFilter_WithFromTimeAndToTime(t *testing.T) {
	storage := &model.LocalStorage{Path: "/data"}
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	filter := NewPathFilter("some/path", storage).WithFromTime(from).WithToTime(to)

	require.NotNil(t, filter.FromTime)
	require.NotNil(t, filter.ToTime)
	assert.Equal(t, from, *filter.FromTime)
	assert.Equal(t, to, *filter.ToTime)
	assert.Equal(t, from, *filter.timeBounds().FromTime)
	assert.Equal(t, to, *filter.timeBounds().ToTime)
}

func TestPathFilter_WithTimeBounds(t *testing.T) {
	storage := &model.LocalStorage{Path: "/data"}
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	bounds := model.TimeBounds{FromTime: &from, ToTime: &to}

	filter := NewPathFilter("some/path", storage).WithTimeBounds(bounds)

	assert.Equal(t, &from, filter.FromTime)
	assert.Equal(t, &to, filter.ToTime)
}

func TestPathFilter_String(t *testing.T) {
	storage := &model.LocalStorage{Path: "/data"}
	filter := NewPathFilter("some/path", storage)

	str := filter.String()

	assert.Contains(t, str, "path: some/path")
	assert.Contains(t, str, "storage: LocalStorage(Path: /data)")
	assert.Contains(t, str, "timebounds: NA")
}

func TestPathFilter_String_WithBounds(t *testing.T) {
	storage := &model.LocalStorage{Path: "/data"}
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	filter := NewPathFilter("some/path", storage).WithFromTime(from)

	str := filter.String()

	assert.Contains(t, str, "path: some/path")
	assert.NotContains(t, str, "timebounds: NA")
}
