package dto

const (
	// max possible value https://aerospike.com/docs/server/reference/configuration#namespace__rack-id
	maxRack = 1000000

	// We need to enforce a minimum bandwidth due to rate limiter constraints.
	// Set to 8 MiB/s for the maximum record size.
	minBandwidth = 0
)
