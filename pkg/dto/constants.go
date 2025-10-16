package dto

import "time"

const (
	// max possible value https://aerospike.com/docs/server/reference/configuration#namespace__rack-id
	maxRack    = 1000000
	maxTimeout = int64(24 * time.Hour / 1e6)
)
