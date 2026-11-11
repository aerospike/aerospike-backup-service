package dto

import (
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
)

// TestNewResultFromModel gives every counter its own value, so that two fields that swap their
// sources fail the test.
func TestNewResultFromModel(t *testing.T) {
	t.Parallel()

	status := &model.RestoreJobStatus{
		Status: model.RestoreFailure,
		Error:  errors.New("restore failed"),
		Counters: model.RestoreCounters{
			ReadRecords:      1,
			InsertedRecords:  2,
			SkippedRecords:   3,
			ExpiredRecords:   4,
			IgnoredRecords:   5,
			ExistedRecords:   6,
			FresherRecords:   7,
			ErrorsInDoubt:    8,
			TotalBytesRead:   9,
			BytesWritten:     10, // not part of the API
			SecondaryIndexes: 11,
			UDFs:             12,
		},
	}

	assert.Equal(t, &RestoreJobStatus{
		ReadRecords:     1,
		InsertedRecords: 2,
		SkippedRecords:  3,
		ExpiredRecords:  4,
		IgnoredRecords:  5,
		ExistedRecords:  6,
		FresherRecords:  7,
		ErrorsInDoubt:   8,
		TotalBytes:      9,
		IndexCount:      11,
		UDFCount:        12,
		Status:          NewJobStatusFromModel(model.RestoreFailure),
		Error:           "restore failed",
	}, NewResultFromModel(status))
}

func TestNewResultFromModel_Nil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, NewResultFromModel(nil))
}
