package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestorePolicy_ValidPolicy(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			Parallel:           new(8),
			TotalTimeout:       new(int64(2000)),
			SocketTimeout:      new(int64(1000)),
			MaxAsyncBatches:    new(32),
			BatchSize:          new(128),
			Bandwidth:          new(int64(50000)),
			Tps:                new(4000),
			Replace:            new(true),
			NoGeneration:       new(false),
			ExtraTTL:           new(int64(86400)),
			DisableBatchWrites: new(false),
			NoRecords:          new(false),
			NoIndexes:          new(false),
			NoUdfs:             new(false),
			SetList:            []string{"set1", "set2"},
			BinList:            []string{"bin1", "bin2"},
		},
	}
	err := policy.Validate()
	require.NoError(t, err)
}

func TestRestorePolicy_NilPolicy(t *testing.T) {
	var policy *RestorePolicy
	err := policy.Validate()
	require.NoError(t, err)
}

func TestRestorePolicy_InvalidTotalTimeout(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			TotalTimeout: new(int64(-1)),
		},
	}
	err := policy.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "total-timeout")
	assert.Contains(t, err.Error(), "should not be negative number")
}

func TestRestorePolicy_InvalidSocketTimeout(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			SocketTimeout: new(int64(-1)),
		},
	}
	err := policy.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "socket-timeout")
	assert.Contains(t, err.Error(), "should not be negative number")
}

func TestRestorePolicy_SocketTimeoutZero(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			SocketTimeout: new(int64(0)),
		},
	}
	err := policy.Validate()
	require.NoError(t, err)
}

func TestRestorePolicy_InvalidParallel(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			Parallel: new(0),
		},
	}
	err := policy.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parallel")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_InvalidMaxAsyncBatches(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			MaxAsyncBatches: new(0),
		},
	}
	err := policy.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max-async-batches")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_InvalidBatchSize(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			BatchSize: new(0),
		},
	}
	err := policy.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch-size")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_InvalidBandwidth(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			Bandwidth: new(int64(-1)),
		},
	}
	err := policy.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bandwidth")
	assert.Contains(t, err.Error(), "\"bandwidth\" -1 invalid, should not be negative number")
}

func TestRestorePolicy_InvalidTps(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			Tps: new(0),
		},
	}
	err := policy.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tps")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_MutuallyExclusiveReplaceUnique(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			Replace: new(true),
			Unique:  new(true),
		},
	}
	err := policy.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive fields")
	assert.Contains(t, err.Error(), "replace")
	assert.Contains(t, err.Error(), "unique")
}

func TestRestorePolicy_InvalidExtraTTL(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			ExtraTTL: new(int64(-1)),
		},
	}
	err := policy.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extra-ttl")
	assert.Contains(t, err.Error(), "should not be negative")
}
