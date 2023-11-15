package model

import (
	"encoding/json"
	"errors"
)

// RestoreRequest represents a restore operation request.
type RestoreRequest struct {
	Host                 *string  `json:"host,omitempty"`
	Port                 *int32   `json:"port,omitempty"`
	UseServicesAlternate *bool    `json:"use-services-alternate,omitempty"`
	User                 *string  `json:"user,omitempty"`
	Password             *string  `json:"password,omitempty"`
	Parallel             *uint32  `json:"parallel,omitempty"`
	NoRecords            *bool    `json:"no-records,omitempty"`
	NoIndexes            *bool    `json:"no-indexes,omitempty"`
	NoUdfs               *bool    `json:"no-udfs,omitempty"`
	Timeout              *uint32  `json:"timeout,omitempty"`
	DisableBatchWrites   *bool    `json:"disable-batch-writes,omitempty"`
	MaxAsyncBatches      *uint32  `json:"max-async-batches,omitempty"`
	BatchSize            *uint32  `json:"batch-size,omitempty"`
	Directory            *string  `json:"directory,omitempty"`
	File                 *string  `json:"file,omitempty"`
	S3Region             *string  `json:"s3-region,omitempty"`
	S3Profile            *string  `json:"s3-profile,omitempty"`
	S3EndpointOverride   *string  `json:"s3-endpoint-override,omitempty"`
	S3LogLevel           *string  `json:"s3-log-level,omitempty"`
	NsList               []string `json:"ns-list,omitempty"`
	SetList              []string `json:"set-list,omitempty"`
	BinList              []string `json:"bin-list,omitempty"`
	Replace              *bool    `json:"replace,omitempty"`
	Unique               *bool    `json:"unique,omitempty"`
	NoGeneration         *bool    `json:"no-generation,omitempty"`
	Bandwidth            *uint64  `json:"bandwidth,omitempty"`
	Tps                  *uint32  `json:"tps,omitempty"`
	AuthMode             *string  `json:"auth-mode,omitempty"`
}

// Validate validates the restore operation request.
func (r *RestoreRequest) Validate() error {
	if r.Directory != nil && r.File != nil {
		return errors.New("both restore directory and file are specified")
	}
	if r.Directory == nil && r.File == nil {
		return errors.New("none of directory or file is specified")
	}
	if r.Host == nil {
		return errors.New("host is not specified")
	}
	if r.Port == nil {
		return errors.New("port is not specified")
	}
	return nil
}

// String satisfies the fmt.Stringer interface.
func (r RestoreRequest) String() string {
	request, err := json.Marshal(r)
	if err != nil {
		return err.Error()
	}
	return string(request)
}
