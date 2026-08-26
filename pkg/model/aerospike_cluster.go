package model

import (
	"fmt"
	"slices"
	"time"
)

// AerospikeCluster represents the configuration for an Aerospike cluster for backup.
type AerospikeCluster struct {
	// The cluster name.
	ClusterLabel string
	// The seed nodes details.
	SeedNodes []SeedNode
	// The connection timeout.
	ConnTimeout *time.Duration
	// Whether should use "services-alternate" instead of "services" in info request during cluster tending.
	UseServicesAlternate *bool
	// The authentication details to the Aerospike cluster.
	Credentials *Credentials
	// The cluster TLS configuration.
	TLS *TLS
	// Specifies the maximum number of parallel scans per the cluster.
	MaxParallelScans *int
	// PreferRacks defines the list of acceptable racks in order of preference.
	PreferRacks []int
}

// GetUser safely returns the username.
func (c *AerospikeCluster) GetUser() string {
	if c.Credentials != nil {
		return c.Credentials.User
	}
	return ""
}

// GetPassword returns the configured password.
// Note: This returns the raw password configuration. If using Secret Agent or file path,
// this needs to be resolved by the service layer.
func (c *AerospikeCluster) GetPassword() string {
	if c.Credentials == nil {
		return ""
	}
	return c.Credentials.Password
}

// Hash returns a unique identifier for the AerospikeCluster.
func (c *AerospikeCluster) Hash() uint64 {
	if c == nil {
		return 0
	}

	nodeHashes := make([]uint64, len(c.SeedNodes))
	for i, node := range c.SeedNodes {
		nodeHashes[i] = node.Hash()
	}
	slices.Sort(nodeHashes)

	return hashValues(
		c.ClusterLabel,
		nodeHashes,
		c.ConnTimeout,
		c.UseServicesAlternate,
		c.Credentials.Hash(),
		c.TLS.Hash(),
		c.MaxParallelScans,
		c.PreferRacks,
	)
}

// Credentials represents authentication details to the Aerospike cluster.
type Credentials struct {
	// The username for the cluster authentication.
	User string
	// The password for the cluster authentication.
	// It can be either plain text or path into the secret agent.
	Password string
	// The file path with the password string, will take precedence over the password field.
	PasswordPath string
	// The authentication mode (INTERNAL, EXTERNAL, PKI). Nil means unset.
	AuthMode *AuthMode
	// The name of the configured Secret Agent to use for authentication.
	SecretAgent *SecretAgent
}

// AuthModeOrDefault returns the configured auth mode, or the default when unset.
func (c *Credentials) AuthModeOrDefault() AuthMode {
	if c == nil || c.AuthMode == nil || *c.AuthMode == "" {
		return *defaultConfig.credentials.AuthMode
	}

	return *c.AuthMode
}

// Hash returns a unique identifier for the Credentials.
func (c *Credentials) Hash() uint64 {
	if c == nil {
		return 0
	}

	return hashValues(
		c.User,
		c.Password,
		c.PasswordPath,
		c.AuthMode,
		c.SecretAgent.Hash(),
	)
}

// SeedNode represents details of a node in the Aerospike cluster.
type SeedNode struct {
	// The host name of the node.
	HostName string
	// The port of the node.
	Port Port
	// TLS name sent as SNI and checked against the server certificate.
	TLSName string
}

// Hash returns a unique identifier for the SeedNode.
func (s SeedNode) Hash() uint64 {
	return hashValues(s.HostName, s.TLSName, s.Port)
}

// String returns a string representation of the SeedNode.
func (s SeedNode) String() string {
	return fmt.Sprintf("%s:%s:%d", s.HostName, s.TLSName, s.Port)
}
