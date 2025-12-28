package dto

// VersionResponse represents the application version information.
type VersionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build-time"`
}
