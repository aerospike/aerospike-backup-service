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

var connectionTypes = []ConnectionType{ConnectionTypeTCP, ConnectionTypeUnix}

// Validate checks that the connection type is supported.
func (c ConnectionType) Validate() error {
	canon, ok := canonicalEnum(c, connectionTypes)
	if !ok || canon == "" {
		return errValidationInvalidValue("connection-type", c, connectionTypes)
	}

	return nil
}

// ToModel converts the DTO connection type to the model type.
func (c ConnectionType) ToModel() model.ConnectionType {
	canon, _ := canonicalEnum(c, connectionTypes)
	return model.ConnectionType(canon)
}

// NewConnectionTypeFromModel creates a DTO connection type from the model type.
func NewConnectionTypeFromModel(m model.ConnectionType) ConnectionType {
	return ConnectionType(m)
}
