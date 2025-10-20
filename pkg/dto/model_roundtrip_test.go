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

	runRoundTripTest[*RetryPolicy, model.RetryPolicy](
		t,
		func(rt *rapid.T) *RetryPolicy { return ptr.Of(genDTO.Draw(rt, "retry-policy")) },
		newRetryPolicyFromModel,
	)
}

func Test_Compression_RoundTrip(t *testing.T) {
	genDTO := RapidStruct[CompressionPolicy](defaultRegistry())

	runRoundTripTest[*CompressionPolicy, model.CompressionPolicy](
		t,
		func(rt *rapid.T) *CompressionPolicy { return ptr.Of(genDTO.Draw(rt, "compression")) },
		newCompressionPolicyFromModel,
	)
}
