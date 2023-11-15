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
	HTTPServer        *HTTPServerConfig            `yaml:"service,omitempty" json:"service,omitempty"`
	AerospikeClusters map[string]*AerospikeCluster `yaml:"aerospike-clusters,omitempty" json:"aerospike-clusters,omitempty"`
	Storages          map[string]*Storage          `yaml:"storages,omitempty" json:"storages,omitempty"`
	BackupPolicies    map[string]*BackupPolicy     `yaml:"backup-policies,omitempty" json:"backup-policies,omitempty"`
}

// NewConfigWithDefaultValues returns a new Config with default values.
func NewConfigWithDefaultValues() *Config {
	config := &Config{}
	config.HTTPServer = &HTTPServerConfig{
		Address: "0.0.0.0",
		Port:    8080,
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
	Address string            `yaml:"address,omitempty" json:"address,omitempty"`
	Port    int               `yaml:"port,omitempty" json:"port,omitempty"`
	Rate    RateLimiterConfig `yaml:"rate,omitempty" json:"rate,omitempty"`
}

// RateLimiterConfig represents the service's HTTP server rate limiter configuration.
type RateLimiterConfig struct {
	Tps       int      `yaml:"tps,omitempty" json:"tps,omitempty"`
	Size      int      `yaml:"size,omitempty" json:"size,omitempty"`
	WhiteList []string `yaml:"white-list,omitempty" json:"white-list,omitempty"`
}

// AerospikeCluster represents the configuration for an Aerospike cluster for backup.
type AerospikeCluster struct {
	pwdOnce              sync.Once
	pwd                  *string
	Name                 *string `yaml:"name,omitempty" json:"name,omitempty"`
	Host                 *string `yaml:"host,omitempty" json:"host,omitempty"`
	Port                 *int32  `yaml:"port,omitempty" json:"port,omitempty"`
	UseServicesAlternate *bool   `yaml:"use-services-alternate,omitempty" json:"use-services-alternate,omitempty"`
	User                 *string `yaml:"user,omitempty" json:"user,omitempty"`
	Password             *string `yaml:"password,omitempty" json:"password,omitempty"`
	PasswordPath         *string `yaml:"password-path,omitempty" json:"password-path,omitempty"`
	AuthMode             *string `yaml:"auth-mode,omitempty" json:"auth-mode,omitempty"`
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

// Storage represents the configuration for a backup storage details.
type Storage struct {
	Name               *string      `yaml:"name,omitempty" json:"name,omitempty"`
	Type               *StorageType `yaml:"type,omitempty" json:"type,omitempty"`
	Path               *string      `yaml:"path,omitempty" json:"path,omitempty"`
	S3Region           *string      `yaml:"s3-region,omitempty" json:"s3-region,omitempty"`
	S3Profile          *string      `yaml:"s3-profile,omitempty" json:"s3-profile,omitempty"`
	S3EndpointOverride *string      `yaml:"s3-endpoint-override,omitempty" json:"s3-endpoint-override,omitempty"`
	S3LogLevel         *string      `yaml:"s3-log-level,omitempty" json:"s3-log-level,omitempty"`
}

// BackupPolicy represents a scheduled backup policy.
type BackupPolicy struct {
	Name               *string     `yaml:"name,omitempty" json:"name,omitempty"`
	IntervalMillis     *int64      `yaml:"interval,omitempty" json:"interval,omitempty"`
	IncrIntervalMillis *int64      `yaml:"incr-interval,omitempty" json:"incr-interval,omitempty"`
	BackupType         *BackupType `yaml:"type,omitempty" json:"type,omitempty"`
	SourceCluster      *string     `yaml:"source-cluster,omitempty" json:"source-cluster,omitempty"`
	Storage            *string     `yaml:"storage,omitempty" json:"storage,omitempty"`
	Namespace          *string     `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Parallel           *int32      `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	SetList            []string    `yaml:"set-list,omitempty" json:"set-list,omitempty"`
	BinList            []string    `yaml:"bin-list,omitempty" json:"bin-list,omitempty"`
	NodeList           []Node      `yaml:"node-list,omitempty" json:"node-list,omitempty"`
	SocketTimeout      *uint32     `yaml:"socket-timeout,omitempty" json:"socket-timeout,omitempty"`
	TotalTimeout       *uint32     `yaml:"total-timeout,omitempty" json:"total-timeout,omitempty"`
	MaxRetries         *uint32     `yaml:"max-retries,omitempty" json:"max-retries,omitempty"`
	RetryDelay         *uint32     `yaml:"retry-delay,omitempty" json:"retry-delay,omitempty"`
	RemoveFiles        *bool       `yaml:"remove-files,omitempty" json:"remove-files,omitempty"`
	RemoveArtifacts    *bool       `yaml:"remove-artifacts,omitempty" json:"remove-artifacts,omitempty"`
	NoBins             *bool       `yaml:"no-bins,omitempty" json:"no-bins,omitempty"`
	NoRecords          *bool       `yaml:"no-records,omitempty" json:"no-records,omitempty"`
	NoIndexes          *bool       `yaml:"no-indexes,omitempty" json:"no-indexes,omitempty"`
	NoUdfs             *bool       `yaml:"no-udfs,omitempty" json:"no-udfs,omitempty"`
	Bandwidth          *uint64     `yaml:"bandwidth,omitempty" json:"bandwidth,omitempty"`
	MaxRecords         *uint64     `yaml:"max-records,omitempty" json:"max-records,omitempty"`
	RecordsPerSecond   *uint32     `yaml:"records-per-second,omitempty" json:"records-per-second,omitempty"`
	FileLimit          *uint64     `yaml:"file-limit,omitempty" json:"file-limit,omitempty"`
	PartitionList      *string     `yaml:"partition-list,omitempty" json:"partition-list,omitempty"`
	AfterDigest        *string     `yaml:"after-digest,omitempty" json:"after-digest,omitempty"`
	FilterExp          *string     `yaml:"filter-exp,omitempty" json:"filter-exp,omitempty"`
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
