package dto

import (
	"errors"
	"fmt"
	"io"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/safepath"
)

// AerospikeCluster represents the configuration for an Aerospike cluster for backup.
// @Description AerospikeCluster represents the configuration for an Aerospike cluster for backup.
//
//nolint:lll
type AerospikeCluster struct {
	// The cluster name. Optional: used only in logs and error messages.
	ClusterLabel string `yaml:"label,omitempty" json:"label,omitempty" example:"testCluster" extensions:"x-nullable"`
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
	// Specifies the maximum number of parallel scans allowed across the cluster.
	// This property helps reduce the load on the cluster and is shared among all backups using the cluster.
	// Default: unlimited.
	MaxParallelScans *int `yaml:"max-parallel-scans,omitempty" json:"max-parallel-scans,omitempty" example:"100" extensions:"x-nullable"`
	// The list of acceptable racks in order of preference.
	// Nodes in prefer-racks[0] are chosen first.
	// If a node is not found in prefer-racks[0], then nodes in prefer-racks[1] are searched, and so on.
	// Mutually exclusive with the routine's rack-list, node-list and partition-list properties.
	PreferRacks []int `yaml:"prefer-racks,omitempty" json:"prefer-racks,omitempty" extensions:"x-nullable"`
}

// Validate validates the Aerospike cluster entity.
func (a *AerospikeCluster) Validate(opts ValidationOptions) error {
	if a == nil {
		return errors.New("cluster is not specified")
	}
	if len(a.SeedNodes) == 0 {
		return errors.New("seed nodes are not specified")
	}
	if duplicates := collections.CheckDuplicates(a.SeedNodes); len(duplicates) > 0 {
		return errValidationDuplicate("seed-nodes", duplicates)
	}

	nodeOpts := opts
	if a.TLS != nil {
		nodeOpts = opts.With(ValidationWithTLS)
	}

	for _, node := range a.SeedNodes {
		if err := node.Validate(nodeOpts); err != nil {
			return err
		}
	}
	if err := a.validateSeedNodesTLSConsistency(); err != nil {
		return err
	}

	if err := a.Credentials.Validate(opts); err != nil {
		return fmt.Errorf("credentials validation error: %w", err)
	}

	tlsOpts := opts
	if a.Credentials != nil && a.Credentials.hasSecretAgent() {
		tlsOpts = opts.With(ValidationWithSecretAgent)
	}

	if err := a.TLS.Validate(tlsOpts); err != nil {
		return fmt.Errorf("tls validation error: %w", err)
	}

	if duplicates := collections.CheckDuplicates(a.PreferRacks); len(duplicates) > 0 {
		return errValidationDuplicate("prefer-racks", duplicates)
	}
	for i, rack := range a.PreferRacks {
		if rack < 0 {
			return errValidationNegative(fmt.Sprintf("prefer-racks[%d]", i), rack)
		}
		if rack > maxRack {
			return fmt.Errorf("rack id %d invalid, should not exceed %d", rack, maxRack)
		}
	}

	if a.MaxParallelScans != nil && *a.MaxParallelScans < 1 {
		return errValidationNonPositive("max-parallel-scans", *a.MaxParallelScans)
	}

	return nil
}

// validateSeedNodesTLSConsistency ensures that if any seed node has TLS configuration,
// then all seed nodes must have TLS configuration.
func (a *AerospikeCluster) validateSeedNodesTLSConsistency() error {
	if len(a.SeedNodes) <= 1 {
		return nil
	}

	var hasTLSNodes, hasNonTLSNodes bool

	for _, node := range a.SeedNodes {
		if node.TLSName != "" {
			hasTLSNodes = true
		} else {
			hasNonTLSNodes = true
		}
	}

	if hasTLSNodes && hasNonTLSNodes {
		return errors.New("if any seed node has TLS configuration (tls-name), all seed nodes must have TLS configuration")
	}

	return nil
}

// NewClusterFromReader creates a new Storage object from a given reader.
func NewClusterFromReader(r io.Reader, format decoder.SerializationFormat) (*AerospikeCluster, error) {
	a := &AerospikeCluster{}
	if err := decoder.Deserialize(a, r, format); err != nil {
		return nil, err
	}

	if err := a.Validate(ValidationDefault); err != nil {
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
	a.PreferRacks = m.PreferRacks
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
		PreferRacks:          a.PreferRacks,
	}, nil
}

func (a *AerospikeCluster) seedNodesToModel() []model.SeedNode {
	nodes := make([]model.SeedNode, len(a.SeedNodes))
	for i, d := range a.SeedNodes {
		nodes[i] = d.toModel()
	}
	return nodes
}

// Credentials represents authentication details to the Aerospike cluster.
// @Description Credentials represents authentication details to the Aerospike cluster.
//
//nolint:lll
type Credentials struct {
	SecretAgentConfig `yaml:",inline"`
	// The username for the cluster authentication.
	User string `yaml:"user,omitempty" json:"user,omitempty" example:"testUser"  extensions:"x-nullable"`
	// The password for the cluster authentication.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	// Literal values are redacted as "[secret]" in API responses; secret agent references are returned as-is.
	Password secret `yaml:"password,omitempty" json:"password,omitempty" format:"password" extensions:"x-nullable"`
	// The file path with the password string.
	PasswordPath string `yaml:"password-path,omitempty" json:"password-path,omitempty" example:"/path/to/pass.txt"  extensions:"x-nullable"`
	// The authentication mode (INTERNAL, EXTERNAL, PKI).
	AuthMode AuthMode `yaml:"auth-mode,omitempty" json:"auth-mode,omitempty" default:"INTERNAL"`
}

func (c *Credentials) fromModel(m *model.Credentials, config *model.BackupConfig) {
	c.User = m.User
	c.Password = secret(m.Password)
	c.PasswordPath = m.PasswordPath
	c.AuthMode = NewAuthModeFromModel(m.AuthMode)

	c.SecretAgentConfig = ResolveSecretAgentFromModel(m.SecretAgent, config)
}

// Validate validates the credentials configuration.
func (c *Credentials) Validate(opts ValidationOptions) error {
	if c == nil {
		return nil
	}

	hasAuth := c.Password != "" || c.PasswordPath != ""

	if hasAuth && c.User == "" {
		return errors.New("username is required when using authentication")
	}

	if c.Password != "" && c.PasswordPath != "" {
		return errValidationMutuallyExclusive("password", "password-path")
	}

	if err := safepath.ValidateClean(c.PasswordPath); err != nil {
		return fmt.Errorf("%w: invalid password-path", errInvalidPath)
	}

	if err := c.AuthMode.Validate(); err != nil {
		return err
	}

	withAgent := c.hasSecretAgent()
	if err := c.Password.Validate(withAgent); err != nil {
		return errValidationSecret("password", err)
	}

	//nolint:staticcheck // We want to call embedded methods with embedded struct name.
	return c.SecretAgentConfig.validate(opts)
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
		Password:     string(c.Password),
		PasswordPath: c.PasswordPath,
		AuthMode:     c.AuthMode.ToModel(),
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
func (node *SeedNode) Validate(opts ValidationOptions) error {
	if opts.Has(ValidationWithTLS) && node.TLSName == "" {
		return errValidationEmptyField("tls-name")
	}
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
