package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestRestorePolicy_ValidPolicy(t *testing.T) {
	policy := &RestorePolicy{
		Parallel:           util.Ptr(8),
		TotalTimeout:       util.Ptr(int64(2000)),
		SocketTimeout:      util.Ptr(int64(1000)),
		MaxAsyncBatches:    util.Ptr(32),
		BatchSize:          util.Ptr(128),
		Bandwidth:          util.Ptr(50000),
		Tps:                util.Ptr(4000),
		Replace:            util.Ptr(true),
		NoGeneration:       util.Ptr(false),
		ExtraTTL:           util.Ptr(int64(86400)),
		DisableBatchWrites: util.Ptr(false),
		NoRecords:          util.Ptr(false),
		NoIndexes:          util.Ptr(false),
		NoUdfs:             util.Ptr(false),
		SetList:            []string{"set1", "set2"},
		BinList:            []string{"bin1", "bin2"},
	}
	err := policy.Validate()
	assert.NoError(t, err)
}

func TestRestorePolicy_NilPolicy(t *testing.T) {
	var policy *RestorePolicy
	err := policy.Validate()
	assert.NoError(t, err)
}

func TestRestorePolicy_InvalidTotalTimeout(t *testing.T) {
	policy := &RestorePolicy{
		TotalTimeout: util.Ptr(int64(-1)),
	}
	err := policy.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "total-timeout")
	assert.Contains(t, err.Error(), "should not be negative number")
}

func TestRestorePolicy_InvalidSocketTimeout(t *testing.T) {
	policy := &RestorePolicy{
		SocketTimeout: util.Ptr(int64(-1)),
	}
	err := policy.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "socket-timeout")
	assert.Contains(t, err.Error(), "should not be negative number")
}

func TestRestorePolicy_SocketTimeoutZero(t *testing.T) {
	policy := &RestorePolicy{
		SocketTimeout: util.Ptr(int64(0)),
	}
	err := policy.Validate()
	assert.NoError(t, err)
}

func TestRestorePolicy_InvalidParallel(t *testing.T) {
	policy := &RestorePolicy{
		Parallel: util.Ptr(0),
	}
	err := policy.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parallel")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_InvalidMaxAsyncBatches(t *testing.T) {
	policy := &RestorePolicy{
		MaxAsyncBatches: util.Ptr(0),
	}
	err := policy.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max-async-batches")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_InvalidBatchSize(t *testing.T) {
	policy := &RestorePolicy{
		BatchSize: util.Ptr(0),
	}
	err := policy.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch-size")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_InvalidBandwidth(t *testing.T) {
	policy := &RestorePolicy{
		Bandwidth: util.Ptr(0),
	}
	err := policy.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bandwidth")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_InvalidTps(t *testing.T) {
	policy := &RestorePolicy{
		Tps: util.Ptr(0),
	}
	err := policy.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tps")
	assert.Contains(t, err.Error(), "should be positive number")
}

func TestRestorePolicy_MutuallyExclusiveReplaceUnique(t *testing.T) {
	policy := &RestorePolicy{
		Replace: util.Ptr(true),
		Unique:  util.Ptr(true),
	}
	err := policy.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive fields")
	assert.Contains(t, err.Error(), "replace")
	assert.Contains(t, err.Error(), "unique")
}

func TestRestorePolicy_InvalidExtraTTL(t *testing.T) {
	policy := &RestorePolicy{
		ExtraTTL: util.Ptr(int64(-1)),
	}
	err := policy.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extra-ttl")
	assert.Contains(t, err.Error(), "should not be negative")
}
