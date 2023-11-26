package model

import (
	"encoding/json"
	"errors"
)

// RestoreRequest represents a restore operation request.
type RestoreRequest struct {
	DestinationCuster *AerospikeCluster `json:"destination,omitempty"`
	SourceStorage     *Storage          `json:"source,omitempty"`
	Policy            *RestorePolicy    `json:"policy,omitempty"`
	Directory         *string           `json:"directory,omitempty"`
	File              *string           `json:"file,omitempty"`
	Time              int64             `json:"time,omitempty" format:"int64"`
	Routine           string            `json:"routine,omitempty"`
}

type RestorePolicy struct {
	Parallel           *uint32  `json:"parallel,omitempty"`
	NoRecords          *bool    `json:"no-records,omitempty"`
	NoIndexes          *bool    `json:"no-indexes,omitempty"`
	NoUdfs             *bool    `json:"no-udfs,omitempty"`
	Time               int64    `json:"time,omitempty" format:"int64"`
	Routine            string   `json:"routine,omitempty"`
	Timeout            *uint32  `json:"timeout,omitempty"`
	DisableBatchWrites *bool    `json:"disable-batch-writes,omitempty"`
	MaxAsyncBatches    *uint32  `json:"max-async-batches,omitempty"`
	BatchSize          *uint32  `json:"batch-size,omitempty"`
	NsList             []string `json:"ns-list,omitempty"`
	SetList            []string `json:"set-list,omitempty"`
	BinList            []string `json:"bin-list,omitempty"`
	Replace            *bool    `json:"replace,omitempty"`
	Unique             *bool    `json:"unique,omitempty"`
	NoGeneration       *bool    `json:"no-generation,omitempty"`
	Bandwidth          *uint64  `json:"bandwidth,omitempty"`
	Tps                *uint32  `json:"tps,omitempty"`
}

// Validate validates the restore operation request.
func (r *RestoreRequest) Validate() error {
	if err := r.DestinationCuster.Validate(); err != nil {
		return err
	}
	if r.Directory != nil && r.File != nil {
		return errors.New("both restore directory and file are specified")
	}
	if r.Directory == nil && r.File == nil {
		return errors.New("none of directory or file is specified")
	}
	if r.Time == 0 {
		return errors.New("restore point in time is not specified")
	}
	if r.Routine == "" {
		return errors.New("routine to restore is not specified")
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
