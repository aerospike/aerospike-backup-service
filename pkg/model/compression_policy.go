package model

// CompressionPolicy contains backup compression information.
type CompressionPolicy struct {
	// The compression mode to be used (default is NONE).
	Mode string
	// The compression level to use (or -1 if unspecified).
	// This field is ignored if the compression mode is NONE.
	// This field is ignored during restoration.
	Level int32
}
