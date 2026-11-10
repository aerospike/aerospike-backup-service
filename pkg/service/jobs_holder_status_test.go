package service

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSumRestoreRuns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		runs         []model.RestoreRun
		wantRead     uint64
		wantProgress *float64
	}{
		{
			name: "counters add up, and progress follows from them",
			runs: []model.RestoreRun{
				{Counters: model.RestoreCounters{ReadRecords: 1}},
				{Counters: model.RestoreCounters{ReadRecords: 2}},
			},
			wantRead: 3,
		},
		{
			name:         "the mean of the progress every run reports",
			runs:         []model.RestoreRun{{Progress: new(0.2)}, {Progress: new(0.6)}},
			wantProgress: new(0.4),
		},
		{
			name: "no progress unless every run reports one",
			runs: []model.RestoreRun{{Progress: new(0.2)}, {}},
		},
		{name: "no runs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			handlers := make([]RestoreHandler, 0, len(tt.runs))
			for _, run := range tt.runs {
				handler := NewMockRestoreHandler(ctrl)
				handler.EXPECT().Status().Return(run)
				handlers = append(handlers, handler)
			}

			sum := sumRestoreRuns(handlers)

			assert.Equal(t, tt.wantRead, sum.Counters.ReadRecords)
			if tt.wantProgress == nil {
				assert.Nil(t, sum.Progress)
				return
			}
			assert.InDelta(t, *tt.wantProgress, *sum.Progress, 1e-9)
		})
	}
}
