package dto

import (
	"testing"

	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricsFromModel_Nil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, NewMetricsFromModel(nil))
}

func TestNewMetricsFromModel(t *testing.T) {
	t.Parallel()

	m := &models.Metrics{
		RecordsPerSecond:       100,
		KilobytesPerSecond:     200,
		PipelineReadQueueSize:  3,
		PipelineWriteQueueSize: 4,
	}

	metrics := NewMetricsFromModel(m)
	require.NotNil(t, metrics)
	assert.Equal(t, uint64(100), metrics.RecordsPerSecond)
	assert.Equal(t, uint64(200), metrics.KilobytesPerSecond)
	assert.Equal(t, 7, metrics.Pipeline)
}
