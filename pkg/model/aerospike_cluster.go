package model

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup/pkg/util"
)

// AerospikeCluster represents the configuration for an Aerospike cluster for backup.
// @Description AerospikeCluster represents the configuration for an Aerospike cluster for backup.
type AerospikeCluster struct {
	pwdOnce sync.Once
	pwd     *string
	// The cluster name.
	ClusterLabel *string `yaml:"label,omitempty" json:"label,omitempty"`
	// The seed nodes details.
	SeedNodes []SeedNode `yaml:"seed-nodes,omitempty" json:"seed-nodes,omitempty"`
	// The connection timeout in milliseconds.
	ConnTimeout *int32 `yaml:"conn-timeout,omitempty" json:"conn-timeout,omitempty"`
	// Whether should use "services-alternate" instead of "services" in info request during cluster tending.
	UseServicesAlternate *bool `yaml:"use-services-alternate,omitempty" json:"use-services-alternate,omitempty"`
	// The authentication details to the Aerospike cluster.
	Credentials *Credentials `yaml:"credentials,omitempty" json:"credentials,omitempty"`
	// The cluster TLS configuration.
	TLS *TLS `yaml:"tls,omitempty" json:"tls,omitempty"`
}

// NewLocalAerospikeCluster returns a new AerospikeCluster to be used in tests.
func NewLocalAerospikeCluster() *AerospikeCluster {
	return &AerospikeCluster{
		SeedNodes:   []SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &Credentials{User: util.Ptr("tester"), Password: util.Ptr("psw")},
	}
}

// SeedNodesAsString returns string representation of the seed nodes list.
func (c *AerospikeCluster) SeedNodesAsString() *string {
	nodes := make([]string, 0, len(c.SeedNodes))
	for _, node := range c.SeedNodes {
		nodes = append(nodes, node.String())
	}
	concatenated := strings.Join(nodes, ",")
	return &concatenated
}

// GetUser safely returns the username.
func (c *AerospikeCluster) GetUser() *string {
	if c.Credentials != nil {
		return c.Credentials.User
	}
	return nil
}

// GetPassword tries to read and set the password once from PasswordPath, if it exists.
// Returns the password value.
func (c *AerospikeCluster) GetPassword() *string {
	c.pwdOnce.Do(func() {
		if c.Credentials != nil {
			if c.Credentials.PasswordPath != nil {
				data, err := os.ReadFile(*c.Credentials.PasswordPath)
				if err != nil {
					slog.Error("Failed to read password", "path", *c.Credentials.PasswordPath, "err", err)
				} else {
					slog.Debug("Successfully read password", "path", *c.Credentials.PasswordPath)
					password := string(data)
					c.pwd = &password
				}
			} else {
				c.pwd = c.Credentials.Password
			}
		}
	})
	return c.pwd
}

// GetAuthMode safely returns the authentication mode.
func (c *AerospikeCluster) GetAuthMode() *string {
	if c.Credentials != nil {
		return c.Credentials.AuthMode
	}
	return nil
}

// Validate validates the Aerospike cluster entity.
func (c *AerospikeCluster) Validate() error {
	if c == nil {
		return errors.New("cluster is not specified")
	}
	if len(c.SeedNodes) == 0 {
		return errors.New("seed nodes are not specified")
	}
	for _, node := range c.SeedNodes {
		if err := node.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ASClientPolicy builds and returns a new ClientPolicy from the AerospikeCluster configuration.
func (c *AerospikeCluster) ASClientPolicy() *as.ClientPolicy {
	policy := as.NewClientPolicy()
	if c.Credentials != nil {
		policy.User = util.ValueOrZero(c.Credentials.User)
		policy.Password = util.ValueOrZero(c.GetPassword())
		if c.Credentials.AuthMode != nil {
			switch strings.ToUpper(*c.Credentials.AuthMode) {
			case "INTERNAL":
				policy.AuthMode = as.AuthModeInternal
			case "EXTERNAL":
				policy.AuthMode = as.AuthModeExternal
			case "PKI":
				policy.AuthMode = as.AuthModePKI
			}
		}
	}
	if c.ConnTimeout != nil {
		policy.Timeout = time.Duration(*c.ConnTimeout) * time.Millisecond
	}
	if c.UseServicesAlternate != nil {
		policy.UseServicesAlternate = *c.UseServicesAlternate
	}
	return policy
}

// ASClientHosts builds and returns a Host list from the AerospikeCluster configuration.
func (c *AerospikeCluster) ASClientHosts() []*as.Host {
	hosts := make([]*as.Host, 0, len(c.SeedNodes))
	for _, node := range c.SeedNodes {
		hosts = append(hosts, &as.Host{
			Name:    node.HostName,
			Port:    int(node.Port),
			TLSName: node.TLSName,
		})
	}
	return hosts
}

// TLS represents the Aerospike cluster TLS configuration options.
// @Description TLS represents the Aerospike cluster TLS configuration options.
type TLS struct {
	// Path to a trusted CA certificate file.
	CAFile *string `yaml:"cafile,omitempty" json:"cafile,omitempty"`
	// Path to a directory of trusted CA certificates.
	CAPath *string `yaml:"capath,omitempty" json:"capath,omitempty"`
	// The default TLS name used to authenticate each TLS socket connection.
	Name *string `yaml:"name,omitempty" json:"name,omitempty"`
	// TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.
	Protocols *string `yaml:"protocols,omitempty" json:"protocols,omitempty"`
	// TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format.
	CipherSuite *string `yaml:"cipher-suite,omitempty" json:"cipher-suite,omitempty"`
	// Path to the key for mutual authentication (if Aerospike cluster supports it).
	Keyfile *string `yaml:"keyfile,omitempty" json:"keyfile,omitempty"`
	// Password to load protected TLS-keyfile (env:VAR, file:PATH, PASSWORD).
	KeyfilePassword *string `yaml:"keyfile-password,omitempty" json:"keyfile-password,omitempty"`
	// Path to the chain file for mutual authentication (if Aerospike Cluster supports it).
	Certfile *string `yaml:"certfile,omitempty" json:"certfile,omitempty"`
	// Path to a certificate blocklist file. The file should contain one line for each blocklisted certificate.
	CertBlacklist *string `yaml:"cert-blacklist,omitempty" json:"cert-blacklist,omitempty"`
}

// Credentials represents authentication details to the Aerospike cluster.
// @Description Credentials represents authentication details to the Aerospike cluster.
type Credentials struct {
	// The username for the cluster authentication.
	User *string `yaml:"user,omitempty" json:"user,omitempty"`
	// The password for the cluster authentication.
	Password *string `yaml:"password,omitempty" json:"password,omitempty"`
	// The file path with the password string, will take precedence over the password field.
	PasswordPath *string `yaml:"password-path,omitempty" json:"password-path,omitempty"`
	// The authentication mode string (INTERNAL, EXTERNAL, EXTERNAL_INSECURE, PKI).
	AuthMode *string `yaml:"auth-mode,omitempty" json:"auth-mode,omitempty"`
}

// SeedNode represents details of a node in the Aerospike cluster.
// @Description SeedNode represents details of a node in the Aerospike cluster.
type SeedNode struct {
	// The host name of the node.
	HostName string `yaml:"host-name,omitempty" json:"host-name,omitempty"`
	// The port of the node.
	Port int32 `yaml:"port,omitempty" json:"port,omitempty"`
	// TLS certificate name used for secure connections (if enabled).
	TLSName string `yaml:"tls-name,omitempty" json:"tls-name,omitempty"`
}

// Validate validates the SeedNode entity.
func (node *SeedNode) Validate() error {
	if node.HostName == "" {
		return errors.New("empty hostname is not allowed")
	}
	if node.Port < 1 || node.Port > 65535 {
		return errors.New("invalid port number")
	}
	return nil
}

// String returns the string representation of the SeedNode.
func (node *SeedNode) String() string {
	if node.TLSName != "" {
		return fmt.Sprintf("%s:%s:%d", node.HostName, node.TLSName, node.Port)
	}
	return fmt.Sprintf("%s:%d", node.HostName, node.Port)
}
