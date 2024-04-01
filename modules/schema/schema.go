//go:build !ci

package schema

import (
	"embed"
)

//go:embed schemas/json/aerospike
var schemas embed.FS
