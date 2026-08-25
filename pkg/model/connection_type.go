package model

// ConnectionType is the Secret Agent connection type.
type ConnectionType string

const (
	ConnectionTypeTCP  ConnectionType = "tcp"
	ConnectionTypeUnix ConnectionType = "unix"
)
