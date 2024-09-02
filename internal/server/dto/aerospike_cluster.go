//nolint:lll
package dto

import (
	"errors"
	"io"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
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

// NewLocalAerospikeCluster returns a new AerospikeCluster to be used in tests.
func NewLocalAerospikeCluster() *AerospikeCluster {
	return &AerospikeCluster{
		SeedNodes:   []SeedNode{{HostName: "localhost", Port: 3000}},
		Credentials: &Credentials{User: util.Ptr("tester"), Password: util.Ptr("psw")},
	}
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

func NewClusterFromModel(m *model.AerospikeCluster) *AerospikeCluster {
	a := &AerospikeCluster{}
	a.fromModel(m)
	return a
}

func (a *AerospikeCluster) fromModel(m *model.AerospikeCluster) {
	a.ClusterLabel = m.ClusterLabel
	a.SeedNodes = make([]SeedNode, len(m.SeedNodes))
	for i, v := range m.SeedNodes {
		a.SeedNodes[i].fromModel(v)
	}
	a.ConnTimeout = m.ConnTimeout
	a.UseServicesAlternate = m.UseServicesAlternate
	a.Credentials = &Credentials{}
	a.Credentials.fromModel(m.Credentials)
	if m.TLS != nil {
		a.TLS = &TLS{}
		a.TLS.fromModel(m.TLS)
	}
	a.MaxParallelScans = m.MaxParallelScans
}

func (a *AerospikeCluster) ToModel() *model.AerospikeCluster {
	return &model.AerospikeCluster{
		ClusterLabel:         a.ClusterLabel,
		SeedNodes:            a.seedNodesToModel(),
		ConnTimeout:          a.ConnTimeout,
		UseServicesAlternate: a.UseServicesAlternate,
		Credentials:          a.Credentials.toModel(),
		TLS:                  a.TLS.toModel(),
		MaxParallelScans:     a.MaxParallelScans,
	}
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
}

func (c *Credentials) fromModel(m *model.Credentials) {
	c.User = m.User
	c.Password = m.Password
	c.PasswordPath = m.PasswordPath
	c.AuthMode = m.AuthMode
}

func (c *Credentials) toModel() *model.Credentials {
	return &model.Credentials{
		User:         c.User,
		Password:     c.Password,
		PasswordPath: c.PasswordPath,
		AuthMode:     c.AuthMode,
	}
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
