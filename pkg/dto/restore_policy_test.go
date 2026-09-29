package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestorePolicy_ValidPolicy(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			Parallel:           ptr.Of(8),
			TotalTimeout:       ptr.Of(int64(2000)),
			SocketTimeout:      ptr.Of(int64(1000)),
			MaxAsyncBatches:    ptr.Of(32),
			BatchSize:          ptr.Of(128),
			Bandwidth:          ptr.Of(int64(50000)),
			Tps:                ptr.Of(4000),
			Replace:            ptr.Of(true),
			NoGeneration:       ptr.Of(false),
			ExtraTTL:           ptr.Of(int64(86400)),
			DisableBatchWrites: ptr.Of(false),
			NoRecords:          ptr.Of(false),
			NoIndexes:          ptr.Of(false),
			NoUdfs:             ptr.Of(false),
			SetList:            []string{"set1", "set2"},
			BinList:            []string{"bin1", "bin2"},
		},
	}
	err := policy.Validate(ValidationDefault)
	require.NoError(t, err)
}

func TestRestorePolicy_NilPolicy(t *testing.T) {
	var policy *RestorePolicy
	err := policy.Validate(ValidationDefault)
	require.NoError(t, err)
}

func TestRestorePolicy_InvalidTotalTimeout(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			TotalTimeout: ptr.Of(int64(-1)),
		},
	}
	err := policy.Validate(ValidationDefault)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "total-timeout")
	assert.Contains(t, err.Error(), "should not be negative number")
}

func TestRestorePolicy_InvalidSocketTimeout(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			SocketTimeout: ptr.Of(int64(-1)),
		},
	}
	err := policy.Validate(ValidationDefault)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "socket-timeout")
	assert.Contains(t, err.Error(), "should not be negative number")
}

func TestRestorePolicy_SocketTimeoutZero(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			SocketTimeout: ptr.Of(int64(0)),
		},
	}
	err := policy.Validate(ValidationDefault)
	require.NoError(t, err)
}

func TestRestorePolicy_InvalidParallel(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			Parallel: ptr.Of(0),
		},
	}
	err := policy.Validate(ValidationDefault)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parallel")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_InvalidMaxAsyncBatches(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			MaxAsyncBatches: ptr.Of(0),
		},
	}
	err := policy.Validate(ValidationDefault)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max-async-batches")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_InvalidBatchSize(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			BatchSize: ptr.Of(0),
		},
	}
	err := policy.Validate(ValidationDefault)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch-size")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_InvalidBandwidth(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			Bandwidth: ptr.Of(int64(-1)),
		},
	}
	err := policy.Validate(ValidationDefault)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bandwidth")
	assert.Contains(t, err.Error(), "\"bandwidth\" -1 invalid, should not be negative number")
}

func TestRestorePolicy_InvalidTps(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			Tps: ptr.Of(0),
		},
	}
	err := policy.Validate(ValidationDefault)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tps")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_MutuallyExclusiveReplaceUnique(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			Replace: ptr.Of(true),
			Unique:  ptr.Of(true),
		},
	}
	err := policy.Validate(ValidationDefault)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive fields")
	assert.Contains(t, err.Error(), "replace")
	assert.Contains(t, err.Error(), "unique")
}

func TestRestorePolicy_InvalidExtraTTL(t *testing.T) {
	policy := &RestorePolicy{
		BaseRestorePolicy: BaseRestorePolicy{
			ExtraTTL: ptr.Of(int64(-1)),
		},
	}
	err := policy.Validate(ValidationDefault)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extra-ttl")
	assert.Contains(t, err.Error(), "should not be negative")
}
