package dto

// ValidationOption holds configuration for validation behavior.
type ValidationOption int

const (
	ValidationSkipTLSFiles ValidationOption = iota
)
