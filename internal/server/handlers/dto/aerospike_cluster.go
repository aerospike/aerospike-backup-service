package dto

import (
	"github.com/aerospike/backup/pkg/model"
)

// AerospikeClusterDTO represents the configuration for an Aerospike cluster for backup.
// @Description AerospikeClusterDTO represents the configuration for an Aerospike cluster for backup.
type AerospikeClusterDTO struct {
	// The cluster name.
	ClusterLabel *string `json:"label,omitempty" example:"testCluster"`
	// The seed nodes details.
	SeedNodes []SeedNodeDTO `json:"seed-nodes,omitempty"`
	// The connection timeout in milliseconds.
	ConnTimeout *int32 `json:"conn-timeout,omitempty" example:"5000"`
	// Whether should use "services-alternate" instead of "services" in info request during cluster tending.
	UseServicesAlternate *bool `json:"use-services-alternate,omitempty"`
	// The authentication details to the Aerospike cluster.
	Credentials *CredentialsDTO `json:"credentials,omitempty"`
	// The cluster TLS configuration.
	TLS *TLSDTO `json:"tls,omitempty"`
	// Specifies the size of the Aerospike Connection Queue per node.
	ConnectionQueueSize *int `json:"connection-queue-size,omitempty" example:"100"`
}

// SeedNodeDTO represents details of a node in the Aerospike cluster.
// @Description SeedNodeDTO represents details of a node in the Aerospike cluster.
type SeedNodeDTO struct {
	// The host name of the node.
	HostName string `json:"host-name,omitempty" example:"localhost" validate:"required"`
	// The port of the node.
	Port int32 `json:"port,omitempty" example:"3000" validate:"required"`
	// TLS certificate name used for secure connections (if enabled).
	TLSName string `json:"tls-name,omitempty" example:"certName" validate:"optional"`
}

// CredentialsDTO represents authentication details to the Aerospike cluster.
// @Description CredentialsDTO represents authentication details to the Aerospike cluster.
type CredentialsDTO struct {
	// The username for the cluster authentication.
	User *string `json:"user,omitempty" example:"testUser"`
	// The password for the cluster authentication.
	Password *string `json:"password,omitempty" example:"testPswd"`
	// The file path with the password string, will take precedence over the password field.
	PasswordPath *string `json:"password-path,omitempty" example:"/path/to/pass.txt"`
	// The authentication mode string (INTERNAL, EXTERNAL, EXTERNAL_INSECURE, PKI).
	AuthMode *string `json:"auth-mode,omitempty" enums:"INTERNAL,EXTERNAL,EXTERNAL_INSECURE,PKI"`
}

// TLSDTO represents the Aerospike cluster TLS configuration options.
// @Description TLSDTO represents the Aerospike cluster TLS configuration options.
type TLSDTO struct {
	// Path to a trusted CA certificate file.
	CAFile *string `json:"cafile,omitempty" example:"/path/to/cafile.pem"`
	// Path to a directory of trusted CA certificates.
	CAPath *string `json:"capath,omitempty" example:"/path/to/ca"`
	// The default TLS name used to authenticate each TLS socket connection.
	Name *string `json:"name,omitempty" example:"tls-name"`
	// TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.
	Protocols *string `json:"protocols,omitempty" example:"TLSv1.2"`
	// TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format.
	CipherSuite *string `json:"cipher-suite,omitempty" example:"ECDHE-ECDSA-AES256-GCM-SHA384"`
	// Path to the key for mutual authentication (if Aerospike cluster supports it).
	Keyfile *string `json:"keyfile,omitempty" example:"/path/to/keyfile.pem"`
	// Password to load protected TLS-keyfile (env:VAR, file:PATH, PASSWORD).
	KeyfilePassword *string `json:"keyfile-password,omitempty" example:"file:/path/to/password"`
	// Path to the chain file for mutual authentication (if Aerospike Cluster supports it).
	Certfile *string `json:"certfile,omitempty" example:"/path/to/certfile.pem"`
}

// MapAerospikeClusterFromDTO maps AerospikeClusterDTO to model.AerospikeCluster.
func MapAerospikeClusterFromDTO(dto AerospikeClusterDTO) model.AerospikeCluster {
	return model.AerospikeCluster{
		ClusterLabel:         dto.ClusterLabel,
		SeedNodes:            mapSeedNodesFromDTOs(dto.SeedNodes),
		ConnTimeout:          dto.ConnTimeout,
		UseServicesAlternate: dto.UseServicesAlternate,
		Credentials: &model.Credentials{
			User:         dto.Credentials.User,
			Password:     dto.Credentials.Password,
			PasswordPath: dto.Credentials.PasswordPath,
			AuthMode:     dto.Credentials.AuthMode,
		},
		TLS: &model.TLS{
			CAFile:          dto.TLS.CAFile,
			CAPath:          dto.TLS.CAPath,
			Name:            dto.TLS.Name,
			Protocols:       dto.TLS.Protocols,
			CipherSuite:     dto.TLS.CipherSuite,
			Keyfile:         dto.TLS.Keyfile,
			KeyfilePassword: dto.TLS.KeyfilePassword,
			Certfile:        dto.TLS.Certfile,
		},
		ConnectionQueueSize: dto.ConnectionQueueSize,
	}
}

func mapSeedNodesFromDTO(dto SeedNodeDTO) model.SeedNode {
	return model.SeedNode{
		HostName: dto.HostName,
		Port:     dto.Port,
		TLSName:  dto.TLSName,
	}
}

func mapSeedNodesFromDTOs(dtos []SeedNodeDTO) []model.SeedNode {
	result := make([]model.SeedNode, 0, len(dtos))
	for i := range dtos {
		result = append(result, mapSeedNodesFromDTO(dtos[i]))
	}
	return result
}

// MapAerospikeClusterToDTO maps  model.AerospikeCluster to AerospikeClusterDTO.
func MapAerospikeClusterToDTO(c model.AerospikeCluster) AerospikeClusterDTO {
	dto := AerospikeClusterDTO{
		ClusterLabel:         c.ClusterLabel,
		SeedNodes:            mapSeedNodesToDTOs(c.SeedNodes),
		ConnTimeout:          c.ConnTimeout,
		UseServicesAlternate: c.UseServicesAlternate,
		ConnectionQueueSize:  c.ConnectionQueueSize,
	}

	if c.Credentials != nil {
		dto.Credentials = &CredentialsDTO{
			User:         c.Credentials.User,
			Password:     c.Credentials.Password,
			PasswordPath: c.Credentials.PasswordPath,
			AuthMode:     c.Credentials.AuthMode,
		}
	}

	if c.TLS != nil {
		dto.TLS = &TLSDTO{
			CAFile:          c.TLS.CAFile,
			CAPath:          c.TLS.CAPath,
			Name:            c.TLS.Name,
			Protocols:       c.TLS.Protocols,
			CipherSuite:     c.TLS.CipherSuite,
			Keyfile:         c.TLS.Keyfile,
			KeyfilePassword: c.TLS.KeyfilePassword,
			Certfile:        c.TLS.Certfile,
		}
	}

	return dto
}

func mapSeedNodesToDTO(n model.SeedNode) SeedNodeDTO {
	return SeedNodeDTO{
		HostName: n.HostName,
		Port:     n.Port,
		TLSName:  n.TLSName,
	}
}

func mapSeedNodesToDTOs(ns []model.SeedNode) []SeedNodeDTO {
	result := make([]SeedNodeDTO, 0, len(ns))
	for i := range ns {
		result = append(result, mapSeedNodesToDTO(ns[i]))
	}
	return result
}

// MapAerospikeClusterMapsToDTO maps map[string]*model.AerospikeCluster to map[string]*AerospikeClusterDTO.
func MapAerospikeClusterMapsToDTO(m map[string]*model.AerospikeCluster) map[string]*AerospikeClusterDTO {
	result := make(map[string]*AerospikeClusterDTO, len(m))
	for k, v := range m {
		c := MapAerospikeClusterToDTO(*v)
		result[k] = &c
	}
	return result
}
