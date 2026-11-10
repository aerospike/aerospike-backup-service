package dto

import (
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResultFromModel_Nil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, NewResultFromModel(nil))
}

func TestNewResultFromModel(t *testing.T) {
	t.Parallel()

	m := &model.RestoreJobStatus{
		Counters: model.RestoreCounters{ReadRecords: 5, InsertedRecords: 2},
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
