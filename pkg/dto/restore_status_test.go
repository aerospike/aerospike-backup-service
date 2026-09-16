package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobStatusValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  JobStatus
		wantErr bool
	}{
		{
			name:   "running",
			status: RestoreRunning,
		},
		{
			name:   "uppercase",
			status: "SUCCESS",
		},
		{
			name:   "done alias",
			status: "done",
		},
		{
			name:   "failed alias",
			status: "FAILED",
		},
		{
			name:    "unknown",
			status:  "unknown",
			wantErr: true,
		},
		{
			name:    "empty",
			status:  "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.status.Validate()
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestJobStatus_ToModel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, model.RestoreRunning, RestoreRunning.ToModel())
	assert.Equal(t, model.RestoreSuccess, RestoreSuccess.ToModel())
	assert.Equal(t, model.RestoreFailure, RestoreFailure.ToModel())
	assert.Equal(t, model.RestoreCanceled, RestoreCanceled.ToModel())
	assert.Equal(t, model.RestoreRunning, JobStatus("RUNNING").ToModel())
	assert.Equal(t, model.RestoreSuccess, JobStatus("done").ToModel())
	assert.Equal(t, model.RestoreFailure, JobStatus("FAILED").ToModel())
	assert.Equal(t, model.RestoreState("unknown"), JobStatus("unknown").ToModel())
}

func TestNewJobStatusFromModel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, RestoreRunning, NewJobStatusFromModel(model.RestoreRunning))
	assert.Equal(t, RestoreSuccess, NewJobStatusFromModel(model.RestoreSuccess))
	assert.Equal(t, RestoreFailure, NewJobStatusFromModel(model.RestoreFailure))
	assert.Equal(t, RestoreCanceled, NewJobStatusFromModel(model.RestoreCanceled))
	assert.Equal(t, JobStatus(""), NewJobStatusFromModel(""))
}
