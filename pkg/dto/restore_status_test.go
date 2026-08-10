package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestAllJobStatuses(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"running", "success", "failure", "canceled"}, AllJobStatuses())
}

func TestParseJobStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input      string
		wantStatus JobStatus
		wantOK     bool
	}{
		{"running", RestoreRunning, true},
		{"success", RestoreSuccess, true},
		{"done", RestoreSuccess, true},
		{"failure", RestoreFailure, true},
		{"failed", RestoreFailure, true},
		{"canceled", RestoreCanceled, true},
		{"  SUCCESS  ", RestoreSuccess, true},
		{"unknown", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got, ok := ParseJobStatus(tt.input)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantStatus, got)
		})
	}
}

func TestJobStatus_ToModel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, model.RestoreRunning, RestoreRunning.ToModel())
	assert.Equal(t, model.RestoreSuccess, RestoreSuccess.ToModel())
	assert.Equal(t, model.RestoreFailure, RestoreFailure.ToModel())
	assert.Equal(t, model.RestoreCanceled, RestoreCanceled.ToModel())
	assert.Equal(t, model.RestoreState(-1), JobStatus("unknown").ToModel())
}

func TestJobStatusFromModel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, RestoreRunning, JobStatusFromModel(model.RestoreRunning))
	assert.Equal(t, RestoreSuccess, JobStatusFromModel(model.RestoreSuccess))
	assert.Equal(t, RestoreFailure, JobStatusFromModel(model.RestoreFailure))
	assert.Equal(t, RestoreCanceled, JobStatusFromModel(model.RestoreCanceled))
	assert.Equal(t, JobStatus(""), JobStatusFromModel(model.RestoreState(99)))
}
