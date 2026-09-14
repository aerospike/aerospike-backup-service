package dto

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
)

// Secret is the credential type used by every sensitive DTO field. It is an alias, not a
// defined type, so it stays identical to redact.Secret: the reflective walks in
// pkg/dto/decoder recognize it, and no conversion is needed when building pkg/model.
type Secret = redact.Secret

const (
	// max possible value https://aerospike.com/docs/server/reference/configuration#namespace__rack-id
	maxRack    = 1000000
	maxTimeout = int64(24 * time.Hour / 1e6)
)
