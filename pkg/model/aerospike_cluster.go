package model

import (
	"errors"
	"log/slog"
	"os"
	"sync"
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
	// The usename for the cluster authentication.
	User *string `yaml:"user,omitempty" json:"user,omitempty"`
	// The password for the cluster authentication.
	Password *string `yaml:"password,omitempty" json:"password,omitempty"`
	// The file path with the password string, will take precedence over the password field.
	PasswordPath *string `yaml:"password-path,omitempty" json:"password-path,omitempty"`
	// The authentication mode string (INTERNAL, EXTERNAL, EXTERNAL_INSECURE, PKI).
	AuthMode *string `yaml:"auth-mode,omitempty" json:"auth-mode,omitempty"`
}

// GetPassword tries to read and set the password once from PasswordPath, if it exists.
// Returns the password value.
func (c *AerospikeCluster) GetPassword() *string {
	c.pwdOnce.Do(func() {
		if c.PasswordPath != nil {
			data, err := os.ReadFile(*c.PasswordPath)
			if err != nil {
				slog.Error("Failed to read password", "path", *c.PasswordPath)
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
