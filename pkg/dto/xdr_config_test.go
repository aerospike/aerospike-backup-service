package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
)

func TestXDRConfig_RoundTrip(t *testing.T) {
	original := &XDRConfig{
		LocalHost: "127.0.0.1",
		PortRange: &PortRange{
			Start: 3000,
			End:   3005,
		},
		ResultQueueSize: ptr.Of(1000),
		AckQueueSize:    ptr.Of(100),
		MaxConns:        ptr.Of(50),
		ReadTimeout:     ptr.Of[int64](5000),
		WriteTimeout:    ptr.Of[int64](4000),
		StartTimeout:    ptr.Of[int64](3000),
		PollingPeriod:   ptr.Of[int64](60000),
		InfoRetryPolicy: &RetryPolicy{
			BaseTimeout: ptr.Of[int64](100),
			Multiplier:  ptr.Of(2.0),
			MaxRetries:  ptr.Of(3),
		},
	}

	modelConfig := original.ToModel()
	convertedBack := newXDRConfigFromModel(modelConfig)

	assert.Equal(t, original, convertedBack)
}

func TestXDRConfig_RoundTrip_Nil(t *testing.T) {
	var original *XDRConfig
	modelConfig := original.ToModel()
	convertedBack := newXDRConfigFromModel(modelConfig)
	assert.Nil(t, convertedBack)
}
