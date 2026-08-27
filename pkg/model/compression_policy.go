package model

// CompressionMode identifies the compression algorithm used for backup files.
type CompressionMode string

const (
	CompressionModeNone CompressionMode = "NONE"
	CompressionModeZSTD CompressionMode = "ZSTD"
)

// String returns the wire value of the compression mode.
func (m CompressionMode) String() string {
	return string(m)
}

// CompressionPolicy contains backup compression information.
type CompressionPolicy struct {
	// The compression mode to be used (default is NONE).
	Mode CompressionMode
	// The compression level to use (or -1 if unspecified).
	// This field is ignored if the compression mode is NONE.
	// This field is ignored during restoration.
	Level int32
}
