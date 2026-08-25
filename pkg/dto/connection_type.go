package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// ConnectionType is the Secret Agent connection type.
// @Description ConnectionType is the Secret Agent connection type.
type ConnectionType string

const (
	ConnectionTypeTCP  ConnectionType = "tcp"
	ConnectionTypeUnix ConnectionType = "unix"
)

// Validate checks that the connection type is supported.
func (c ConnectionType) Validate() error {
	switch c {
	case ConnectionTypeTCP, ConnectionTypeUnix:
		return nil
	default:
		return errValidationInvalidValue(
			"connection-type",
			c,
			[]ConnectionType{ConnectionTypeTCP, ConnectionTypeUnix},
		)
	}
}

// ToModel converts the DTO connection type to the model type.
func (c ConnectionType) ToModel() model.ConnectionType {
	return model.ConnectionType(c)
}

// NewConnectionTypeFromModel creates a DTO connection type from the model type.
func NewConnectionTypeFromModel(m model.ConnectionType) ConnectionType {
	return ConnectionType(m)
}
