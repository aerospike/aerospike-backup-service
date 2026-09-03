package dto

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
)

type secret = decoder.Secret

const (
	// max possible value https://aerospike.com/docs/server/reference/configuration#namespace__rack-id
	maxRack    = 1000000
	maxTimeout = int64(24 * time.Hour / 1e6)
)
