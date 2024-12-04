//nolint:lll
package dto

import (
	"errors"
	"fmt"
	"io"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// AerospikeCluster represents the configuration for an Aerospike cluster for backup.
// @Description AerospikeCluster represents the configuration for an Aerospike cluster for backup.
type AerospikeCluster struct {
	// The cluster name.
	ClusterLabel *string `yaml:"label,omitempty" json:"label,omitempty" example:"testCluster"`
	// The seed nodes details.
	SeedNodes []SeedNode `yaml:"seed-nodes,omitempty" json:"seed-nodes,omitempty"`
	// The connection timeout in milliseconds.
	ConnTimeout *int32 `yaml:"conn-timeout,omitempty" json:"conn-timeout,omitempty" example:"5000"`
	// Whether should use "services-alternate" instead of "services" in info request during cluster tending.
	UseServicesAlternate *bool `yaml:"use-services-alternate,omitempty" json:"use-services-alternate,omitempty"`
	// The authentication details to the Aerospike cluster.
	Credentials *Credentials `yaml:"credentials,omitempty" json:"credentials,omitempty"`
	// The cluster TLS configuration.
	TLS *TLS `yaml:"tls,omitempty" json:"tls,omitempty"`
	// Specifies the maximum number of parallel scans per the cluster.
	MaxParallelScans *int `yaml:"max-parallel-scans,omitempty" json:"max-parallel-scans,omitempty" example:"100" validate:"optional"`
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

// NewClusterFromReader creates a new Storage object from a given reader
func NewClusterFromReader(r io.Reader, format SerializationFormat) (*AerospikeCluster, error) {
	a := &AerospikeCluster{}
	if err := Deserialize(a, r, format); err != nil {
		return nil, err
	}

	if err := a.Validate(); err != nil {
		return nil, err
	}

	return a, nil
}

func NewClusterFromModel(m *model.AerospikeCluster, config *model.Config) *AerospikeCluster {
	if m == nil {
		return nil
	}

	a := &AerospikeCluster{}
	a.fromModel(m, config)
	return a
}

func (a *AerospikeCluster) fromModel(m *model.AerospikeCluster, config *model.Config) {
	a.ClusterLabel = m.ClusterLabel
	a.SeedNodes = make([]SeedNode, len(m.SeedNodes))
	for i, v := range m.SeedNodes {
		a.SeedNodes[i].fromModel(v)
	}
	a.ConnTimeout = m.ConnTimeout
	a.UseServicesAlternate = m.UseServicesAlternate
	a.Credentials = &Credentials{}
	a.Credentials.fromModel(m.Credentials, config)
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
		ConnTimeout:          a.ConnTimeout,
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
type TLS struct {
	// Path to a trusted CA certificate file.
	CAFile *string `yaml:"ca-file,omitempty" json:"ca-file,omitempty" example:"/path/to/cafile.pem"`
	// Path to a directory of trusted CA certificates.
	CAPath *string `yaml:"ca-path,omitempty" json:"ca-path,omitempty" example:"/path/to/ca"`
	// The default TLS name used to authenticate each TLS socket connection.
	Name *string `yaml:"name,omitempty" json:"name,omitempty" example:"tls-name"`
	// TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.
	Protocols *string `yaml:"protocols,omitempty" json:"protocols,omitempty" example:"TLSv1.2"`
	// TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format.
	CipherSuite *string `yaml:"cipher-suite,omitempty" json:"cipher-suite,omitempty" example:"ECDHE-ECDSA-AES256-GCM-SHA384"`
	// Path to the key for mutual authentication (if Aerospike cluster supports it).
	Keyfile *string `yaml:"key-file,omitempty" json:"key-file,omitempty" example:"/path/to/keyfile.pem"`
	// Password to load protected TLS-keyfile (env:VAR, file:PATH, PASSWORD).
	KeyfilePassword *string `yaml:"key-file-password,omitempty" json:"key-file-password,omitempty" example:"file:/path/to/password"`
	// Path to the chain file for mutual authentication (if Aerospike Cluster supports it).
	Certfile *string `yaml:"cert-file,omitempty" json:"cert-file,omitempty" example:"/path/to/certfile.pem"`
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
	// The username for the cluster authentication.
	User *string `yaml:"user,omitempty" json:"user,omitempty" example:"testUser"`
	// The password for the cluster authentication.
	Password *string `yaml:"password,omitempty" json:"password,omitempty" example:"testPswd"`
	// The file path with the password string, will take precedence over the password field.
	PasswordPath *string `yaml:"password-path,omitempty" json:"password-path,omitempty" example:"/path/to/pass.txt"`
	// The authentication mode string (INTERNAL, EXTERNAL, EXTERNAL_INSECURE, PKI).
	AuthMode *string `yaml:"auth-mode,omitempty" json:"auth-mode,omitempty" enums:"INTERNAL,EXTERNAL,EXTERNAL_INSECURE,PKI"`
	// Secret Agent configuration (optional).
	SecretAgent *SecretAgent `yaml:"secret-agent,omitempty" json:"secret-agent,omitempty"`
	// Secret Agent configuration (optional). Link to one of preconfigured agents.
	SecretAgentName *string `yaml:"secret-agent-name,omitempty" json:"secret-agent-name,omitempty"`
	// The secret keyword in Aerospike Secret Agent containing password.
	// Only applicable when SecretAgent is specified.
	PasswordKeySecret *string `yaml:"password-key-secret,omitempty" json:"password-key-secret,omitempty"`
}

func (c *Credentials) fromModel(m *model.Credentials, config *model.Config) {
	c.User = m.User
	c.Password = m.Password
	c.PasswordPath = m.PasswordPath
	c.PasswordKeySecret = m.PasswordKeySecret
	c.AuthMode = m.AuthMode

	secretAgentName, secretAgent := secretAgentToDto(m.SecretAgent, config)
	c.SecretAgentName = secretAgentName
	c.SecretAgent = secretAgent
}

func secretAgentToDto(s *model.SecretAgent, config *model.Config) (*string, *SecretAgent) {
	secretAgentName := findKeyByValue(config.SecretAgents, s)
	if secretAgentName != "" {
		return &secretAgentName, nil
	}

	return nil, NewSecretAgentFromModel(s)
}

// Validate validates the credentials configuration
func (c *Credentials) Validate() error {
	if c == nil {
		return nil
	}

	hasAuth := c.Password != nil || c.PasswordPath != nil || c.PasswordKeySecret != nil

	if hasAuth && c.User == nil {
		return errors.New("username is required when using authentication")
	}

	methodCount := 0
	if c.Password != nil {
		methodCount++
	}

	if c.PasswordPath != nil {
		methodCount++
	}

	if c.PasswordKeySecret != nil {
		methodCount++
		if err := validateSecretAgent(c.SecretAgent, c.SecretAgentName); err != nil {
			return err
		}
	}

	if methodCount > 1 {
		return fmt.Errorf("only one authentication method must be specified, got %d", methodCount)
	}

	return nil
}

func validateSecretAgent(agent *SecretAgent, name *string) error {
	if agent == nil && name == nil {
		return errors.New("either secret-agent or secret-agent-name must be specified")
	}
	if agent != nil && name != nil {
		return errors.New("secret-agent-name and secret-agent are mutually exclusive")
	}
	if err := agent.validate(); err != nil {
		return fmt.Errorf("secret-agent validation error: %w", err)
	}

	return nil
}

func (c *Credentials) toModel(config *model.Config) (*model.Credentials, error) {
	if c == nil {
		return nil, nil
	}

	agent, err := config.ResolveSecretAgent(c.SecretAgentName, c.SecretAgent.ToModel())
	if err != nil {
		return nil, err
	}

	return &model.Credentials{
		User:              c.User,
		Password:          c.Password,
		PasswordPath:      c.PasswordPath,
		AuthMode:          c.AuthMode,
		PasswordKeySecret: c.PasswordKeySecret,
		SecretAgent:       agent,
	}, nil
}

// SeedNode represents details of a node in the Aerospike cluster.
// @Description SeedNode represents details of a node in the Aerospike cluster.
type SeedNode struct {
	// The host name of the node.
	HostName string `yaml:"host-name,omitempty" json:"host-name,omitempty" example:"localhost" validate:"required"`
	// The port of the node.
	Port int32 `yaml:"port,omitempty" json:"port,omitempty" example:"3000" validate:"required"`
	// TLS certificate name used for secure connections (if enabled).
	TLSName string `yaml:"tls-name,omitempty" json:"tls-name,omitempty" example:"certName" validate:"optional"`
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

func (node *SeedNode) fromModel(m model.SeedNode) {
	node.HostName = m.HostName
	node.Port = m.Port
	node.TLSName = m.TLSName
}

func (node *SeedNode) toModel() model.SeedNode {
	return model.SeedNode{
		HostName: node.HostName,
		Port:     node.Port,
		TLSName:  node.TLSName,
	}
}
