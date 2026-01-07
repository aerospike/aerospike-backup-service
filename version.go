package backup

import _ "embed"

//go:embed VERSION
var Version string

var (
	CommitHash string
	BuildTime  string
)
