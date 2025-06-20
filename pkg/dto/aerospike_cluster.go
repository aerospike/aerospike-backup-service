//nolint:lll
package dto

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// AerospikeCluster represents the configuration for an Aerospike cluster for backup.
// @Description AerospikeCluster represents the configuration for an Aerospike cluster for backup.
//
//nolint:lll
type AerospikeCluster struct {
	// The cluster name. Optional: used only in logs and error messages.
	ClusterLabel *string `yaml:"label,omitempty" json:"label,omitempty" example:"testCluster" extensions:"x-nullable"`
	// The seed nodes details.
	SeedNodes []SeedNode `yaml:"seed-nodes,omitempty" json:"seed-nodes,omitempty" validate:"required"`
	// The connection timeout in milliseconds.
	ConnTimeout *int64 `yaml:"conn-timeout,omitempty" json:"conn-timeout,omitempty" example:"5000" default:"30000"`
	// Whether should use "services-alternate" instead of "services" in info request during cluster tending.
	UseServicesAlternate *bool `yaml:"use-services-alternate,omitempty" json:"use-services-alternate,omitempty" default:"false"`
	// The authentication details to the Aerospike cluster.
	Credentials *Credentials `yaml:"credentials,omitempty" json:"credentials,omitempty"`
	// The cluster TLS configuration.
	TLS *TLS `yaml:"tls,omitempty" json:"tls,omitempty"`
	// Specifies the maximum number of parallel scans per the cluster.
	MaxParallelScans *int `yaml:"max-parallel-scans,omitempty" json:"max-parallel-scans,omitempty" example:"100" extensions:"x-nullable"`
}

// Validate validates the Aerospike cluster entity.
func (a *AerospikeCluster) Validate() error {
	if a == nil {
		return errors.New("cluster is not specified")
	}
	if len(a.SeedNodes) == 0 {
		return errors.New("seed nodes are not specified")
	}
	for _, node := range a.SeedNodes {
		if err := node.Validate(); err != nil {
			return err
		}
	}
	if err := a.Credentials.Validate(); err != nil {
		return fmt.Errorf("credentials validation error: %w", err)
	}

	return nil
}

// NewClusterFromReader creates a new Storage object from a given reader.
func NewClusterFromReader(r io.Reader, format decoder.SerializationFormat) (*AerospikeCluster, error) {
	a := &AerospikeCluster{}
	if err := decoder.Deserialize(a, r, format); err != nil {
		return nil, err
	}

	if err := a.Validate(); err != nil {
		return nil, err
	}

	return a, nil
}

func NewClusterFromModel(m *model.AerospikeCluster, config *model.BackupConfig) *AerospikeCluster {
	if m == nil {
		return nil
	}

	a := &AerospikeCluster{}
	a.fromModel(m, config)
	return a
}

func (a *AerospikeCluster) fromModel(m *model.AerospikeCluster, config *model.BackupConfig) {
	a.ClusterLabel = m.ClusterLabel
	a.SeedNodes = make([]SeedNode, len(m.SeedNodes))
	for i, v := range m.SeedNodes {
		a.SeedNodes[i].fromModel(v)
	}
	a.ConnTimeout = durationToMillis(m.ConnTimeout)
	a.UseServicesAlternate = m.UseServicesAlternate
	if m.Credentials != nil {
		a.Credentials = &Credentials{}
		a.Credentials.fromModel(m.Credentials, config)
	}
	if m.TLS != nil {
		a.TLS = &TLS{}
		a.TLS.fromModel(m.TLS)
	}
	a.MaxParallelScans = m.MaxParallelScans
}

func (a *AerospikeCluster) ToModel(config *model.Config) (*model.AerospikeCluster, error) {
	credentials, err := a.Credentials.toModel(config)
	if err != nil {
		return nil, fmt.Errorf("credentials error: %w", err)
	}

	return &model.AerospikeCluster{
		ClusterLabel:         a.ClusterLabel,
		SeedNodes:            a.seedNodesToModel(),
		ConnTimeout:          millisToDuration(a.ConnTimeout),
		UseServicesAlternate: a.UseServicesAlternate,
		Credentials:          credentials,
		TLS:                  a.TLS.toModel(),
		MaxParallelScans:     a.MaxParallelScans,
	}, nil
}

func (a *AerospikeCluster) seedNodesToModel() []model.SeedNode {
	nodes := make([]model.SeedNode, len(a.SeedNodes))
	for i, d := range a.SeedNodes {
		nodes[i] = d.toModel()
	}
	return nodes
}

// TLS represents the Aerospike cluster TLS configuration options.
// @Description TLS represents the Aerospike cluster TLS configuration options.
//
//nolint:lll
type TLS struct {
	// Path to a trusted CA certificate file.
	CAFile *string `yaml:"ca-file,omitempty" json:"ca-file,omitempty" example:"/path/to/cafile.pem" extensions:"x-nullable"`
	// Path to a directory of trusted CA certificates.
	CAPath *string `yaml:"ca-path,omitempty" json:"ca-path,omitempty" example:"/path/to/ca" extensions:"x-nullable"`
	// The default TLS name used to authenticate each TLS socket connection.
	Name *string `yaml:"name,omitempty" json:"name,omitempty" example:"tls-name" extensions:"x-nullable"`
	// TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.
	Protocols *string `yaml:"protocols,omitempty" json:"protocols,omitempty" default:"TLSv1.2"`
	// TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format.
	CipherSuite *string `yaml:"cipher-suite,omitempty" json:"cipher-suite,omitempty" example:"ECDHE-ECDSA-AES256-GCM-SHA384" extensions:"x-nullable"`
	// Path to the key for mutual authentication (if Aerospike cluster supports it).
	Keyfile *string `yaml:"key-file,omitempty" json:"key-file,omitempty" example:"/path/to/keyfile.pem" extensions:"x-nullable"`
	// Password to load protected TLS-keyfile (env:VAR, file:PATH, PASSWORD).
	KeyfilePassword *string `yaml:"key-file-password,omitempty" json:"key-file-password,omitempty" example:"file:/path/to/password" extensions:"x-nullable"`
	// Path to the chain file for mutual authentication (if Aerospike Cluster supports it).
	Certfile *string `yaml:"cert-file,omitempty" json:"cert-file,omitempty" example:"/path/to/certfile.pem" extensions:"x-nullable"`
}

func (t *TLS) fromModel(m *model.TLS) {
	t.CAFile = m.CAFile
	t.CAPath = m.CAPath
	t.Name = m.Name
	t.Protocols = m.Protocols
	t.CipherSuite = m.CipherSuite
	t.Keyfile = m.Keyfile
	t.KeyfilePassword = m.KeyfilePassword
	t.Certfile = m.Certfile
}

func (t *TLS) toModel() *model.TLS {
	if t == nil {
		return nil
	}

	return &model.TLS{
		CAFile:          t.CAFile,
		CAPath:          t.CAPath,
		Name:            t.Name,
		Protocols:       t.Protocols,
		CipherSuite:     t.CipherSuite,
		Keyfile:         t.Keyfile,
		KeyfilePassword: t.KeyfilePassword,
		Certfile:        t.Certfile,
	}
}

// Credentials represents authentication details to the Aerospike cluster.
// @Description Credentials represents authentication details to the Aerospike cluster.
type Credentials struct {
	SecretAgentConfig `yaml:",inline"`
	// The username for the cluster authentication.
	User *string `yaml:"user,omitempty" json:"user,omitempty" example:"testUser"  extensions:"x-nullable"`
	// The password for the cluster authentication.
	// It can be either plain text or path into the secret agent.
	Password *string `yaml:"password,omitempty" json:"password,omitempty" example:"testPswd"  extensions:"x-nullable"`
	// The file path with the password string.
	PasswordPath *string `yaml:"password-path,omitempty" json:"password-path,omitempty" example:"/path/to/pass.txt"  extensions:"x-nullable"`
	// The authentication mode string (INTERNAL, EXTERNAL, PKI).
	AuthMode *string `yaml:"auth-mode,omitempty" json:"auth-mode,omitempty" enums:"INTERNAL,EXTERNAL,PKI" default:"INTERNAL"`
}

func (c *Credentials) fromModel(m *model.Credentials, config *model.BackupConfig) {
	c.User = m.User
	c.Password = m.Password
	c.PasswordPath = m.PasswordPath
	c.AuthMode = m.AuthMode

	c.SecretAgentConfig = ResolveSecretAgentFromModel(m.SecretAgent, config)
}

// Validate validates the credentials configuration.
func (c *Credentials) Validate() error {
	if c == nil {
		return nil
	}

	hasAuth := c.Password != nil || c.PasswordPath != nil

	if hasAuth && c.User == nil {
		return errors.New("username is required when using authentication")
	}

	if c.Password != nil && c.PasswordPath != nil {
		return errValidationMutuallyExclusive("password", "password-path")
	}

	if c.AuthMode != nil &&
		(strings.ToUpper(*c.AuthMode) != "INTERNAL" &&
			strings.ToUpper(*c.AuthMode) != "EXTERNAL" &&
			strings.ToUpper(*c.AuthMode) != "PKI") {
		return fmt.Errorf("auth-mode %q incorrect, should be one of: INTERNAL,EXTERNAL,PKI", *c.AuthMode)
	}
	//nolint:staticcheck // We want to call embedded methods with embedded struct name.
	return c.SecretAgentConfig.validate()
}

func (c *Credentials) toModel(config *model.Config) (*model.Credentials, error) {
	if c == nil {
		return nil, nil
	}
	//nolint:staticcheck // We want to call embedded methods with embedded struct name.
	agent, err := c.SecretAgentConfig.ToModel(config)
	if err != nil {
		return nil, err
	}

	return &model.Credentials{
		User:         c.User,
		Password:     c.Password,
		PasswordPath: c.PasswordPath,
		AuthMode:     c.AuthMode,
		SecretAgent:  agent,
	}, nil
}

// SeedNode represents details of a node in the Aerospike cluster.
// @Description SeedNode represents details of a node in the Aerospike cluster.
type SeedNode struct {
	// The host name of the node.
	HostName string `yaml:"host-name,omitempty" json:"host-name,omitempty" example:"localhost" validate:"required"`
	// The port of the node.
	Port Port `yaml:"port,omitempty" json:"port,omitempty" example:"3000" validate:"required,min=1,max=65535"`
	// TLS certificate name used for secure connections (if enabled).
	TLSName string `yaml:"tls-name,omitempty" json:"tls-name,omitempty" example:"certName" extensions:"x-nullable"`
}

// Validate validates the SeedNode entity.
func (node *SeedNode) Validate() error {
	if node.HostName == "" {
		return errValidationEmptyField("hostname")
	}
	if err := node.Port.Validate(); err != nil {
		return err
	}
	return nil
}

func (node *SeedNode) fromModel(m model.SeedNode) {
	node.HostName = m.HostName
	node.Port = Port(m.Port)
	node.TLSName = m.TLSName
}

func (node *SeedNode) toModel() model.SeedNode {
	return model.SeedNode{
		HostName: node.HostName,
		Port:     model.Port(node.Port),
		TLSName:  node.TLSName,
	}
}
