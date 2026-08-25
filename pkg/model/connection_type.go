package model

// ConnectionType is the Secret Agent connection type.
type ConnectionType string

const (
	ConnectionTypeTCP  ConnectionType = "tcp"
	ConnectionTypeUnix ConnectionType = "unix"
)

// String returns the wire value of the connection type.
func (c ConnectionType) String() string {
	return string(c)
}
