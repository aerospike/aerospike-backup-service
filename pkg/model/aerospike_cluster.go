package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// AerospikeCluster represents the configuration for an Aerospike cluster for backup.
type AerospikeCluster struct {
	// The cluster name.
	ClusterLabel *string
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

// Hash returns a unique string identifier for the AerospikeCluster.
func (c *AerospikeCluster) Hash() string {
	nodeStrings := make([]string, len(c.SeedNodes))
	for i, node := range c.SeedNodes {
		nodeStrings[i] = node.String()
	}
	sort.Strings(nodeStrings)

	// Build a slice of all fields to be hashed.
	hashData := []any{
		ptr.ValueOrZero(c.ClusterLabel),
		nodeStrings,
		ptr.ValueOrZero(c.ConnTimeout).String(),
		ptr.ValueOrZero(c.UseServicesAlternate),
		c.Credentials.String(),
		c.TLS.String(),
		ptr.ValueOrZero(c.MaxParallelScans),
		c.PreferRacks,
	}

	hasher := sha256.New()
	_, _ = fmt.Fprintf(hasher, "%v", hashData)

	return hex.EncodeToString(hasher.Sum(nil))
}

// ToString returns a user-friendly cluster identifier.
// It prefers ClusterLabel; if missing, it uses a stable first seed node string.
func (c *AerospikeCluster) ToString() string {
	if c == nil {
		return ""
	}

	if label := ptr.ValueOrZero(c.ClusterLabel); label != "" {
		return label
	}

	if len(c.SeedNodes) > 0 {
		nodeStrings := make([]string, len(c.SeedNodes))
		for i, node := range c.SeedNodes {
			nodeStrings[i] = node.String()
		}
		sort.Strings(nodeStrings)
		return nodeStrings[0]
	}

	return ""
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

// String returns a string representation of the Credentials.
func (c *Credentials) String() string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("%v:%v:%v:%v:%v",
		c.User,
		c.Password,
		c.PasswordPath,
		c.AuthModeOrDefault(),
		c.SecretAgent.String())
}

// SeedNode represents details of a node in the Aerospike cluster.
type SeedNode struct {
	// The host name of the node.
	HostName string
	// The port of the node.
	Port Port
	// TLS certificate name used for secure connections (if enabled).
	TLSName string
}

// String returns a string representation of the SeedNode.
func (s SeedNode) String() string {
	return fmt.Sprintf("%s:%s:%d", s.HostName, s.TLSName, s.Port)
}
