package model

// BackupPolicy represents a scheduled backup policy.
type BackupPolicy struct {
	Parallel         *int32  `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	SocketTimeout    *uint32 `yaml:"socket-timeout,omitempty" json:"socket-timeout,omitempty"`
	TotalTimeout     *uint32 `yaml:"total-timeout,omitempty" json:"total-timeout,omitempty"`
	MaxRetries       *uint32 `yaml:"max-retries,omitempty" json:"max-retries,omitempty"`
	RetryDelay       *uint32 `yaml:"retry-delay,omitempty" json:"retry-delay,omitempty"`
	RemoveFiles      *bool   `yaml:"remove-files,omitempty" json:"remove-files,omitempty"`
	RemoveArtifacts  *bool   `yaml:"remove-artifacts,omitempty" json:"remove-artifacts,omitempty"`
	NoBins           *bool   `yaml:"no-bins,omitempty" json:"no-bins,omitempty"`
	NoRecords        *bool   `yaml:"no-records,omitempty" json:"no-records,omitempty"`
	NoIndexes        *bool   `yaml:"no-indexes,omitempty" json:"no-indexes,omitempty"`
	NoUdfs           *bool   `yaml:"no-udfs,omitempty" json:"no-udfs,omitempty"`
	Bandwidth        *uint64 `yaml:"bandwidth,omitempty" json:"bandwidth,omitempty"`
	MaxRecords       *uint64 `yaml:"max-records,omitempty" json:"max-records,omitempty"`
	RecordsPerSecond *uint32 `yaml:"records-per-second,omitempty" json:"records-per-second,omitempty"`
	FileLimit        *uint64 `yaml:"file-limit,omitempty" json:"file-limit,omitempty"`
	FilterExp        *string `yaml:"filter-exp,omitempty" json:"filter-exp,omitempty"`
}
