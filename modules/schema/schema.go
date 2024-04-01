//go:build !ci

package schema

import (
	"embed"
)

// this file is copied from the aerospike kubernetes operator

//go:embed schemas/json/aerospike
var schemas embed.FS
