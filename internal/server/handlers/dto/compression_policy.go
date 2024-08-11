package dto

// CompressionPolicyDTO contains backup compression information.
// @Description CompressionPolicyDTO contains backup compression information.
type CompressionPolicyDTO struct {
	// The compression mode to be used (default is NONE).
	Mode string `json:"mode,omitempty" default:"NONE" enums:"NONE,ZSTD"`
	// The compression level to use (or -1 if unspecified).
	Level int32 `json:"level,omitempty"`
}
