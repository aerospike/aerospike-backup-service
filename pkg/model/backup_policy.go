package model

import "github.com/aerospike/backup/pkg/util"

const (
	KeepAll           RemoveFilesType = "KeepAll"
	RemoveAll         RemoveFilesType = "RemoveAll"
	RemoveIncremental RemoveFilesType = "RemoveIncremental"
)

// RemoveFilesType represents the type of the backup storage.
// @Description RemoveFilesType represents the type of the backup storage.
type RemoveFilesType string

// BackupPolicy represents a scheduled backup policy.
// @Description BackupPolicy represents a scheduled backup policy.
//
//nolint:lll
type BackupPolicy struct {
	Parallel         *int32           `yaml:"parallel,omitempty" json:"parallel,omitempty" example:"1"`
	SocketTimeout    *uint32          `yaml:"socket-timeout,omitempty" json:"socket-timeout,omitempty" example:"1000"`
	TotalTimeout     *uint32          `yaml:"total-timeout,omitempty" json:"total-timeout,omitempty" example:"2000"`
	MaxRetries       *uint32          `yaml:"max-retries,omitempty" json:"max-retries,omitempty" example:"3"`
	RetryDelay       *uint32          `yaml:"retry-delay,omitempty" json:"retry-delay,omitempty" example:"500"`
	RemoveFiles      *RemoveFilesType `yaml:"remove-files,omitempty" json:"remove-files,omitempty" enums:"KeepAll,RemoveAll,RemoveIncremental"`
	RemoveArtifacts  *bool            `yaml:"remove-artifacts,omitempty" json:"remove-artifacts,omitempty"`
	NoBins           *bool            `yaml:"no-bins,omitempty" json:"no-bins,omitempty"`
	NoRecords        *bool            `yaml:"no-records,omitempty" json:"no-records,omitempty"`
	NoIndexes        *bool            `yaml:"no-indexes,omitempty" json:"no-indexes,omitempty"`
	NoUdfs           *bool            `yaml:"no-udfs,omitempty" json:"no-udfs,omitempty"`
	Bandwidth        *uint64          `yaml:"bandwidth,omitempty" json:"bandwidth,omitempty" example:"10000"`
	MaxRecords       *uint64          `yaml:"max-records,omitempty" json:"max-records,omitempty" example:"10000"`
	RecordsPerSecond *uint32          `yaml:"records-per-second,omitempty" json:"records-per-second,omitempty" example:"1000"`
	FileLimit        *uint64          `yaml:"file-limit,omitempty" json:"file-limit,omitempty" example:"1024"`
	FilterExp        *string          `yaml:"filter-exp,omitempty" json:"filter-exp,omitempty" example:"EjRWeJq83vEjRRI0VniavN7xI0U="`
}

// CopySMDDisabled creates a new instance of the BackupPolicy struct with identical field values.
// New instance has NoIndexes and NoUdfs set to true.
func (p *BackupPolicy) CopySMDDisabled() *BackupPolicy {
	return &BackupPolicy{
		Parallel:         p.Parallel,
		SocketTimeout:    p.SocketTimeout,
		TotalTimeout:     p.TotalTimeout,
		MaxRetries:       p.MaxRetries,
		RetryDelay:       p.RetryDelay,
		RemoveFiles:      p.RemoveFiles,
		RemoveArtifacts:  p.RemoveArtifacts,
		NoBins:           p.NoBins,
		NoRecords:        p.NoRecords,
		NoIndexes:        util.Ptr(true),
		NoUdfs:           util.Ptr(true),
		Bandwidth:        p.Bandwidth,
		MaxRecords:       p.MaxRecords,
		RecordsPerSecond: p.RecordsPerSecond,
		FileLimit:        p.FileLimit,
		FilterExp:        p.FilterExp,
	}
}

func (r *RemoveFilesType) RemoveFullBackup() bool {
	// Full backups are deleted only if RemoveFiles is explicitly set to RemoveAll
	return r != nil && *r == RemoveAll
}

func (r *RemoveFilesType) RemoveIncrementalBackup() bool {
	// Incremental backups are deleted only if RemoveFiles is explicitly set to RemoveAll or RemoveIncremental
	return r != nil && (*r == RemoveIncremental || *r == RemoveAll)
}
