package model

import saClient "github.com/aerospike/backup-go/pkg/secret-agent"

// ConnectionType is the Secret Agent connection type.
type ConnectionType string

const (
	ConnectionTypeTCP  ConnectionType = "TCP"
	ConnectionTypeUnix ConnectionType = "UNIX"
)

func (c ConnectionType) ResolveType() *string {
	connectionType := saClient.ConnectionTypeTCP
	if c == ConnectionTypeUnix {
		connectionType = saClient.ConnectionTypeUDS
	}

	return &connectionType
}
