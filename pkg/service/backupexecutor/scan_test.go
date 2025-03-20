package backupexecutor

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/stretchr/testify/assert"
)

var now = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func TestCalculateSocketTimeout(t *testing.T) {
	tests := []struct {
		name         string
		routine      *model.BackupRoutine
		isFullBackup bool
		expected     time.Duration
	}{
		{
			name: "explicit timeout less than other constraints",
			routine: &model.BackupRoutine{
				IntervalCron: "@daily",
				BackupPolicy: &model.BackupPolicy{
					SocketTimeout: util.Ptr(5 * time.Minute),
				},
			},
			isFullBackup: true,
			expected:     5 * time.Minute,
		},
		{
			name: "next backup soon",
			routine: &model.BackupRoutine{
				IntervalCron: "0 */1 * * * *", // every minute
				BackupPolicy: &model.BackupPolicy{
					SocketTimeout: util.Ptr(2 * time.Minute),
				},
			},
			isFullBackup: true,
			expected:     time.Minute,
		},
		{
			name: "default cap of 10 minutes applies",
			routine: &model.BackupRoutine{
				IntervalCron: "@daily",
				BackupPolicy: &model.BackupPolicy{
					SocketTimeout: nil, // No explicit timeout
				},
			},
			isFullBackup: true,
			expected:     model.DefaultSocketTimeout,
		},
		{
			name: "user set 0",
			routine: &model.BackupRoutine{
				IntervalCron: "@daily",
				BackupPolicy: &model.BackupPolicy{
					SocketTimeout: util.Ptr(0 * time.Second),
				},
			},
			isFullBackup: true,
			expected:     model.DefaultSocketTimeout,
		},
		{
			name: "incremental backup",
			routine: &model.BackupRoutine{
				IncrIntervalCron: "@hourly",
				BackupPolicy: &model.BackupPolicy{
					SocketTimeout: util.Ptr(3 * time.Minute),
				},
			},
			isFullBackup: false,
			expected:     3 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSocketTimeout(tt.routine, tt.isFullBackup, now)
			assert.Equal(t, result, tt.expected)
		})
	}
}
