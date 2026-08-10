package dto

import (
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResultFromModel_Nil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, NewResultFromModel(nil))
}

func TestNewResultFromModel(t *testing.T) {
	t.Parallel()

	stats := models.NewRestoreStats()
	stats.ReadRecords.Add(5)
	stats.IncrRecordsInserted()
	stats.IncrRecordsInserted()

	m := &model.RestoreJobStatus{
		Counters: stats,
		Status:   model.RestoreFailure,
		Error:    errors.New("boom"),
	}

	result := NewResultFromModel(m)
	require.NotNil(t, result)
	assert.Equal(t, uint64(5), result.ReadRecords)
	assert.Equal(t, uint64(2), result.InsertedRecords)
	assert.Equal(t, RestoreFailure, result.Status)
	assert.Equal(t, "boom", result.Error)
	assert.Nil(t, result.CurrentRestore)
}
