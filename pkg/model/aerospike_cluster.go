package model

import (
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"

	as "github.com/aerospike/aerospike-client-go/v7"
)

// AerospikeCluster represents the configuration for an Aerospike cluster for backup.
// @Description AerospikeCluster represents the configuration for an Aerospike cluster for backup.
type AerospikeCluster struct {
	pwdOnce sync.Once
	pwd     *string
	// The host that acts as the entry point to the cluster.
	Host *string `yaml:"host,omitempty" json:"host,omitempty"`
	// The port to connect to.
	Port *int32 `yaml:"port,omitempty" json:"port,omitempty"`
	// Whether should use "services-alternate" instead of "services" in info request during cluster tending.
	UseServicesAlternate *bool `yaml:"use-services-alternate,omitempty" json:"use-services-alternate,omitempty"`
	// The username for the cluster authentication.
	User *string `yaml:"user,omitempty" json:"user,omitempty"`
	// The password for the cluster authentication.
	Password *string `yaml:"password,omitempty" json:"password,omitempty"`
	// The file path with the password string, will take precedence over the password field.
	PasswordPath *string `yaml:"password-path,omitempty" json:"password-path,omitempty"`
	// The authentication mode string (INTERNAL, EXTERNAL, EXTERNAL_INSECURE, PKI).
	AuthMode *string `yaml:"auth-mode,omitempty" json:"auth-mode,omitempty"`
	// The cluster TLS configuration.
	TLS *TLS `yaml:"tls,omitempty" json:"tls,omitempty"`
}

// GetPassword tries to read and set the password once from PasswordPath, if it exists.
// Returns the password value.
func (c *AerospikeCluster) GetPassword() *string {
	c.pwdOnce.Do(func() {
		if c.PasswordPath != nil {
			data, err := os.ReadFile(*c.PasswordPath)
			if err != nil {
				slog.Error("Failed to read password", "path", *c.PasswordPath, "err", err)
			} else {
				slog.Debug("Successfully read password", "path", *c.PasswordPath)
				password := string(data)
				c.pwd = &password
			}
		} else {
			c.pwd = c.Password
		}
	})
	return c.pwd
}

// Validate validates the Aerospike cluster entity.
func (c *AerospikeCluster) Validate() error {
	if c == nil {
		return errors.New("cluster is not specified")
	}
	if c.Host == nil || len(*c.Host) == 0 {
		return errors.New("host is not specified")
	}
	if c.Port == nil {
		return errors.New("port is not specified")
	}
	return nil
}

// ASClientPolicy builds and returns a new ClientPolicy from the AerospikeCluster configuration.
func (c *AerospikeCluster) ASClientPolicy() *as.ClientPolicy {
	policy := as.NewClientPolicy()
	policy.User = *c.User
	policy.Password = *c.GetPassword()
	if c.AuthMode != nil {
		switch strings.ToUpper(*c.AuthMode) {
		case "INTERNAL":
			policy.AuthMode = as.AuthModeInternal
		case "EXTERNAL":
			policy.AuthMode = as.AuthModeExternal
		case "PKI":
			policy.AuthMode = as.AuthModePKI
		}
	}
	if c.UseServicesAlternate != nil {
		policy.UseServicesAlternate = *c.UseServicesAlternate
	}
	return policy
}

// ASClientHost builds and returns a new Host from the AerospikeCluster configuration.
func (c *AerospikeCluster) ASClientHost() *as.Host {
	return as.NewHost(*c.Host, int(*c.Port))
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
