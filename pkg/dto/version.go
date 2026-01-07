package dto

// VersionResponse represents the ABS version information.
// @Description VersionResponse represents the ABS version information.
type VersionResponse struct {
	// Version of the application.
	Version string `json:"version"`

	// Commit hash of the build.
	Commit string `json:"commit"`

	// BuildTime of the application in UTC.
	BuildTime string `json:"build-time" `
}
