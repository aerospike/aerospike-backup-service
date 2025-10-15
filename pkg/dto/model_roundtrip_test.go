package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"pgregory.net/rapid"
)

// This test verifies that all valid permutation of a DTO struct can be converted to model and back lossless.

func Test_RetryPolicy_RoundTrip(t *testing.T) {
	genDTO := RapidStruct[RetryPolicy](defaultRegistry())

	RoundTrip[*RetryPolicy, model.RetryPolicy](t, func(t *rapid.T) *RetryPolicy {
		return ptr.Of(genDTO.Draw(t, "retry-policy"))
	}, newRetryPolicyFromModel)
}

func Test_PortRange_RoundTrip(t *testing.T) {
	genDTO := RapidStruct[PortRange](defaultRegistry())

	RoundTrip[*PortRange, model.PortRange](t, func(t *rapid.T) *PortRange {
		return ptr.Of(genDTO.Draw(t, "port"))
	}, newPortRangeFromModel)
}

func Test_Compression_RoundTrip(t *testing.T) {
	genDTO := RapidStruct[CompressionPolicy](defaultRegistry())

	RoundTrip[*CompressionPolicy, model.CompressionPolicy](t, func(t *rapid.T) *CompressionPolicy {
		return ptr.Of(genDTO.Draw(t, "port"))
	}, newCompressionPolicyFromModel)
}
