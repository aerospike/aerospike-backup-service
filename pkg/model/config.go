package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/aerospike/backup/internal/util"
	"gopkg.in/yaml.v3"
)

// Config represents the service configuration file.
type Config struct {
	AerospikeClusters []*AerospikeCluster `yaml:"aerospike-cluster,omitempty" json:"aerospike-cluster,omitempty"`
	BackupStorage     []*BackupStorage    `yaml:"backup-storage,omitempty" json:"backup-storage,omitempty"`
	BackupPolicy      []*BackupPolicy     `yaml:"backup-policy,omitempty" json:"backup-policy,omitempty"`
}

// String satisfies the fmt.Stringer interface.
func (c Config) String() string {
	cfg, err := json.Marshal(c)
	if err != nil {
		return err.Error()
	}
	return string(cfg)
}

// AerospikeCluster represents the configuration for an Aerospike cluster for backup.
type AerospikeCluster struct {
	Name     *string `yaml:"name,omitempty" json:"name,omitempty"`
	Host     *string `yaml:"host,omitempty" json:"host,omitempty"`
	Port     *int    `yaml:"port,omitempty" json:"port,omitempty"`
	User     *string `yaml:"user,omitempty" json:"user,omitempty"`
	Password *string `yaml:"password,omitempty" json:"password,omitempty"`
}

// BackupStorage represents the configuration for a backup storage details.
type BackupStorage struct {
	Name               *string      `yaml:"name,omitempty" json:"name,omitempty"`
	Type               *StorageType `yaml:"type,omitempty" json:"type,omitempty"`
	Path               *string      `yaml:"path,omitempty" json:"path,omitempty"`
	S3Region           *string      `yaml:"s3_region,omitempty" json:"s3_region,omitempty"`
	S3Profile          *string      `yaml:"s3_profile,omitempty" json:"s3_profile,omitempty"`
	S3EndpointOverride *string      `yaml:"s3_endpoint_override,omitempty" json:"s3_endpoint_override,omitempty"`
	S3LogLevel         *string      `yaml:"s3_log_level,omitempty" json:"s3_log_level,omitempty"`
}

// BackupPolicy represents a scheduled backup policy.
type BackupPolicy struct {
	Name               *string     `yaml:"name,omitempty" json:"name,omitempty"`
	IntervalMillis     *int64      `yaml:"interval,omitempty" json:"interval,omitempty"`
	IncrIntervalMillis *int64      `yaml:"incr_interval,omitempty" json:"incr_interval,omitempty"`
	BackupType         *BackupType `yaml:"type,omitempty" json:"type,omitempty"`
	SourceCluster      *string     `yaml:"source_cluster,omitempty" json:"source_cluster,omitempty"`
	Storage            *string     `yaml:"storage,omitempty" json:"storage,omitempty"`
	Namespace          *string     `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Parallelism        *int        `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	SetList            *[]string   `yaml:"set_list,omitempty" json:"set_list,omitempty"`
	NodeList           *string     `yaml:"node_list,omitempty" json:"node_list,omitempty"`
	BinList            *string     `yaml:"bin_list,omitempty" json:"bin_list,omitempty"`
	RemoveFiles        *bool       `yaml:"remove_files,omitempty" json:"remove_files,omitempty"`
	RemoveArtifacts    *bool       `yaml:"remove_artifacts,omitempty" json:"remove_artifacts,omitempty"`
	NoBins             *bool       `yaml:"no_bins,omitempty" json:"no_bins,omitempty"`
	NoRecords          *bool       `yaml:"no_records,omitempty" json:"no_records,omitempty"`
	NoIndexes          *bool       `yaml:"no_indexes,omitempty" json:"no_indexes,omitempty"`
	NoUdfs             *bool       `yaml:"no_udfs,omitempty" json:"no_udfs,omitempty"`
}

// Clone clones the backup policy struct.
func (p *BackupPolicy) Clone() *BackupPolicy {
	serialized, err := json.Marshal(p)
	util.Check(err)

	clone := BackupPolicy{}
	err = json.Unmarshal(serialized, &clone)
	util.Check(err)

	return &clone
}

type StorageType int

const (
	Local StorageType = iota
	S3
)

type BackupType int

const (
	Full BackupType = iota
	Incremental
)

// ReadConfiguration reads the configuration from the given file path.
func ReadConfiguration(filePath string) (*Config, error) {
	if filePath == "" {
		return nil, errors.New("configuration file is missing")
	}
	buf, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = yaml.Unmarshal(buf, config)
	if err != nil {
		return nil, fmt.Errorf("in file %q: %w", filePath, err)
	}

	return config, err
}
