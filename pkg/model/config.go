package model

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"

	"github.com/aerospike/backup/internal/util"
)

// Config represents the service configuration file.
type Config struct {
	HTTPServer        *HTTPServerConfig            `yaml:"http-server,omitempty" json:"http-server,omitempty"`
	AerospikeClusters map[string]*AerospikeCluster `yaml:"aerospike-cluster,omitempty" json:"aerospike-cluster,omitempty"`
	BackupStorage     []*BackupStorage             `yaml:"backup-storage,omitempty" json:"backup-storage,omitempty"`
	BackupPolicy      []*BackupPolicy              `yaml:"backup-policy,omitempty" json:"backup-policy,omitempty"`
}

// NewConfigWithDefaultValues returns a new Config with default values.
func NewConfigWithDefaultValues() *Config {
	config := &Config{}
	config.HTTPServer = &HTTPServerConfig{
		Host: "0.0.0.0",
		Port: 8080,
		Rate: RateLimiterConfig{
			Tps:       1024,
			Size:      1024,
			WhiteList: []string{},
		},
	}
	return config
}

// String satisfies the fmt.Stringer interface.
func (c Config) String() string {
	cfg, err := json.Marshal(c)
	if err != nil {
		return err.Error()
	}
	return string(cfg)
}

// HTTPServerConfig represents the service's HTTP server configuration.
type HTTPServerConfig struct {
	Host string            `yaml:"host,omitempty" json:"host,omitempty"`
	Port int               `yaml:"port,omitempty" json:"port,omitempty"`
	Rate RateLimiterConfig `yaml:"rate,omitempty" json:"rate,omitempty"`
}

// RateLimiterConfig represents the service's HTTP server rate limiter configuration.
type RateLimiterConfig struct {
	Tps       int      `yaml:"tps,omitempty" json:"tps,omitempty"`
	Size      int      `yaml:"size,omitempty" json:"size,omitempty"`
	WhiteList []string `yaml:"white_list,omitempty" json:"white_list,omitempty"`
}

// AerospikeCluster represents the configuration for an Aerospike cluster for backup.
type AerospikeCluster struct {
	pwdOnce              sync.Once
	pwd                  *string
	Name                 *string `yaml:"name,omitempty" json:"name,omitempty"`
	Host                 *string `yaml:"host,omitempty" json:"host,omitempty"`
	Port                 *int32  `yaml:"port,omitempty" json:"port,omitempty"`
	UseServicesAlternate *bool   `yaml:"use_services_alternate,omitempty" json:"use_services_alternate,omitempty"`
	User                 *string `yaml:"user,omitempty" json:"user,omitempty"`
	Password             *string `yaml:"password,omitempty" json:"password,omitempty"`
	PasswordPath         *string `yaml:"password_path,omitempty" json:"password_path,omitempty"`
	AuthMode             *string `yaml:"auth_mode,omitempty" json:"auth_mode,omitempty"`
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

// GetName returns the name of the AerospikeCluster.
func (c *AerospikeCluster) GetName() *string {
	return c.Name
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

// GetName returns the name of the BackupStorage.
func (s *BackupStorage) GetName() *string {
	return s.Name
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
	Parallel           *int32      `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	SetList            []string    `yaml:"set_list,omitempty" json:"set_list,omitempty"`
	BinList            []string    `yaml:"bin_list,omitempty" json:"bin_list,omitempty"`
	NodeList           []Node      `yaml:"node_list,omitempty" json:"node_list,omitempty"`
	SocketTimeout      *uint32     `yaml:"socket_timeout,omitempty" json:"socket_timeout,omitempty"`
	TotalTimeout       *uint32     `yaml:"total_timeout,omitempty" json:"total_timeout,omitempty"`
	MaxRetries         *uint32     `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	RetryDelay         *uint32     `yaml:"retry_delay,omitempty" json:"retry_delay,omitempty"`
	RemoveFiles        *bool       `yaml:"remove_files,omitempty" json:"remove_files,omitempty"`
	RemoveArtifacts    *bool       `yaml:"remove_artifacts,omitempty" json:"remove_artifacts,omitempty"`
	NoBins             *bool       `yaml:"no_bins,omitempty" json:"no_bins,omitempty"`
	NoRecords          *bool       `yaml:"no_records,omitempty" json:"no_records,omitempty"`
	NoIndexes          *bool       `yaml:"no_indexes,omitempty" json:"no_indexes,omitempty"`
	NoUdfs             *bool       `yaml:"no_udfs,omitempty" json:"no_udfs,omitempty"`
	Bandwidth          *uint64     `yaml:"bandwidth,omitempty" json:"bandwidth,omitempty"`
	MaxRecords         *uint64     `yaml:"max_records,omitempty" json:"max_records,omitempty"`
	RecordsPerSecond   *uint32     `yaml:"records_per_second,omitempty" json:"records_per_second,omitempty"`
	FileLimit          *uint64     `yaml:"file_limit,omitempty" json:"file_limit,omitempty"`
	PartitionList      *string     `yaml:"partition_list,omitempty" json:"partition_list,omitempty"`
	AfterDigest        *string     `yaml:"after_digest,omitempty" json:"after_digest,omitempty"`
	FilterExp          *string     `yaml:"filter_exp,omitempty" json:"filter_exp,omitempty"`
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

type Node struct {
	IP   string `yaml:"ip" json:"ip"`
	Port int    `yaml:"port" json:"port"`
}

// GetName returns the name of the BackupPolicy.
func (p *BackupPolicy) GetName() *string {
	return p.Name
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

type WithName interface {
	GetName() *string
}
