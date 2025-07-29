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

	"github.com/aerospike/aerospike-backup-service/v3/internal/util/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
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
	hasher := sha256.New()

	if c.ClusterLabel != nil {
		hasher.Write([]byte(*c.ClusterLabel))
		hasher.Write([]byte(":"))
	}

	nodes := make([]SeedNode, len(c.SeedNodes))
	copy(nodes, c.SeedNodes)
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].String() < nodes[j].String()
	})
	for _, node := range nodes {
		hasher.Write([]byte(node.String()))
		hasher.Write([]byte(":"))
	}

	if c.ConnTimeout != nil {
		hasher.Write([]byte(c.ConnTimeout.String()))
		hasher.Write([]byte(":"))
	}

	if c.UseServicesAlternate != nil {
		fmt.Fprintf(hasher, "%v:", *c.UseServicesAlternate)
	}

	if c.Credentials != nil {
		hasher.Write([]byte(c.Credentials.String()))
		hasher.Write([]byte(":"))
	}

	if c.TLS != nil {
		hasher.Write([]byte(c.TLS.String()))
		hasher.Write([]byte(":"))
	}

	if c.MaxParallelScans != nil {
		fmt.Fprintf(hasher, "%d:", *c.MaxParallelScans)
	}

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

// TLS represents the Aerospike cluster TLS configuration options.
type TLS struct {
	// Path to a trusted CA certificate file.
	CAFile *string
	// Path to a directory of trusted CA certificates.
	CAPath *string
	// The default TLS name used to authenticate each TLS socket connection.
	Name *string
	// TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.
	Protocols *string
	// TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format.
	CipherSuite *string
	// Path to the key for mutual authentication (if Aerospike cluster supports it).
	Keyfile *string
	// Password to load protected TLS-keyfile (env:VAR, file:PATH, PASSWORD).
	KeyfilePassword *string
	// Path to the chain file for mutual authentication (if Aerospike Cluster supports it).
	Certfile *string
}

// String returns a string representation of the TLS.
func (tls *TLS) String() string {
	if tls == nil {
		return nilString
	}
	return fmt.Sprintf(
		"%v:%v:%v:%v:%v:%v:%v:%v",
		util.ValueOrZero(tls.CAFile),
		util.ValueOrZero(tls.CAPath),
		util.ValueOrZero(tls.Name),
		util.ValueOrZero(tls.Protocols),
		util.ValueOrZero(tls.CipherSuite),
		util.ValueOrZero(tls.Keyfile),
		util.ValueOrZero(tls.KeyfilePassword),
		util.ValueOrZero(tls.Certfile),
	)
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
		util.ValueOrZero(c.User),
		util.ValueOrZero(c.Password),
		util.ValueOrZero(c.PasswordPath),
		util.ValueOrZero(c.AuthMode),
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
