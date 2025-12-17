package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync/atomic"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

const nilString = "<nil>"

// AerospikeCluster represents the configuration for an Aerospike cluster for backup.
type AerospikeCluster struct {
	pwd atomic.Pointer[string]
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
func (c *AerospikeCluster) GetUser() *string {
	if c.Credentials != nil {
		return c.Credentials.User
	}
	return nil
}

// GetPassword tries to read and set the password once from the configured source.
// Returns the password value. If it fails to read the password, it will return nil
// and try to read again next time.
func (c *AerospikeCluster) GetPassword() *string {
	if password := c.pwd.Load(); password != nil {
		return password
	}

	if c.Credentials == nil {
		return nil
	}

	password := c.Credentials.loadPassword()
	if password != nil {
		c.pwd.Store(password)
	}

	return password
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

func (c *Credentials) loadPassword() *string {
	if c.Password != nil {
		password, err := c.SecretAgent.Read(*c.Password)
		if err != nil {
			slog.Warn("Failed to read password from secret agent", attr.Error(err))
			return nil
		}

		return &password
	}

	if password := c.loadPasswordFromFile(); password != nil {
		return password
	}

	slog.Warn("No valid authentication method configured")

	return nil
}

func (c *Credentials) loadPasswordFromFile() *string {
	if c.PasswordPath == nil {
		return nil
	}

	data, err := os.ReadFile(*c.PasswordPath)
	if err != nil {
		slog.Error("Failed to read password",
			slog.String("path", *c.PasswordPath),
			attr.Error(err))
		return nil
	}

	slog.Debug("Successfully read password", slog.String("path", *c.PasswordPath))
	password := string(data)

	return &password
}

// GetAuthMode safely returns the authentication mode.
func (c *AerospikeCluster) GetAuthMode() *string {
	if c.Credentials != nil {
		return c.Credentials.AuthMode
	}
	return nil
}

// Credentials represents authentication details to the Aerospike cluster.
type Credentials struct {
	// The username for the cluster authentication.
	User *string
	// The password for the cluster authentication.
	// It can be either plain text or path into the secret agent.
	Password *string
	// The file path with the password string, will take precedence over the password field.
	PasswordPath *string
	// The authentication mode string (INTERNAL, EXTERNAL, PKI).
	AuthMode *string
	// The name of the configured Secret Agent to use for authentication.
	SecretAgent *SecretAgent
}

// String returns a string representation of the Credentials.
func (c *Credentials) String() string {
	if c == nil {
		return nilString
	}
	return fmt.Sprintf("%v:%v:%v:%v:%v",
		ptr.ValueOrZero(c.User),
		ptr.ValueOrZero(c.Password),
		ptr.ValueOrZero(c.PasswordPath),
		ptr.ValueOrZero(c.AuthMode),
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
