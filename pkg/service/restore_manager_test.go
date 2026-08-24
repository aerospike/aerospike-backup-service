package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestJobNotFoundError_Error(t *testing.T) {
	err := NewJobNotFoundError(model.RestoreJobID(42))
	assert.Equal(t, "restore job with ID 42 not found", err.Error())
}

func TestRestoreManager_GetFilteredJobs(t *testing.T) {
	jobsHolder := NewRestoreJobsHolder()
	mgr := &restoreManager{restoreJobs: jobsHolder}

	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	jobsHolder.Store(model.RestoreJobID(1), &restoreJob{started: older, status: model.RestoreSuccess})
	jobsHolder.Store(model.RestoreJobID(2), &restoreJob{started: newer, status: model.RestoreFailure})
	jobsHolder.Store(model.RestoreJobID(3), &restoreJob{started: newer, status: model.RestoreRunning})

	t.Run("no filters returns all jobs", func(t *testing.T) {
		results := mgr.GetFilteredJobs(model.TimeBounds{}, model.StatusFilter{})
		assert.Len(t, results, 3)
	})

	t.Run("time bounds filter excludes jobs outside the window", func(t *testing.T) {
		from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
		results := mgr.GetFilteredJobs(model.TimeBounds{FromTime: &from}, model.StatusFilter{})
		assert.Len(t, results, 2)
		assert.NotContains(t, results, model.RestoreJobID(1))
	})

	t.Run("status filter includes only matching statuses", func(t *testing.T) {
		statusFilter := model.NewStatusFilter([]model.RestoreState{model.RestoreSuccess}, false)
		results := mgr.GetFilteredJobs(model.TimeBounds{}, statusFilter)
		assert.Len(t, results, 1)
		assert.Contains(t, results, model.RestoreJobID(1))
	})

	t.Run("status filter in exclude mode omits matching statuses", func(t *testing.T) {
		statusFilter := model.NewStatusFilter([]model.RestoreState{model.RestoreRunning}, true)
		results := mgr.GetFilteredJobs(model.TimeBounds{}, statusFilter)
		assert.Len(t, results, 2)
		assert.NotContains(t, results, model.RestoreJobID(3))
	})

	t.Run("combined time bounds and status filter", func(t *testing.T) {
		from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
		statusFilter := model.NewStatusFilter([]model.RestoreState{model.RestoreFailure}, false)
		results := mgr.GetFilteredJobs(model.TimeBounds{FromTime: &from}, statusFilter)
		assert.Len(t, results, 1)
		assert.Contains(t, results, model.RestoreJobID(2))
	})
}
