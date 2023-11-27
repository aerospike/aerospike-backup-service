package model

import (
	"encoding/json"
	"errors"
)

// RestoreFullRequest represents a full restore operation request.
type RestoreFullRequest struct {
	DestinationCuster *AerospikeCluster `json:"destination,omitempty"`
	SourceStorage     *Storage          `json:"source,omitempty"`
	Policy            *RestorePolicy    `json:"policy,omitempty"`
	Directory         *string           `json:"directory,omitempty"`
}

// RestoreIncrementalRequest represents an incremental restore operation request.
type RestoreIncrementalRequest struct {
	DestinationCuster *AerospikeCluster `json:"destination,omitempty"`
	SourceStorage     *Storage          `json:"source,omitempty"`
	Policy            *RestorePolicy    `json:"policy,omitempty"`
	File              *string           `json:"file,omitempty"`
}

// RestoreTimestampRequest represents a restore by timestamp operation request.
type RestoreTimestampRequest struct {
	DestinationCuster *AerospikeCluster `json:"destination,omitempty"`
	SourceStorage     *Storage          `json:"source,omitempty"`
	Policy            *RestorePolicy    `json:"policy,omitempty"`
	Time              int64             `json:"time,omitempty" format:"int64"`
	Routine           string            `json:"routine,omitempty"`
}

// RestoreRequest represents generic restore request.
// It is only used internally to pass data to restore library.
type RestoreRequest struct {
	DestinationCuster *AerospikeCluster
	SourceStorage     *Storage
	Policy            *RestorePolicy
	Directory         *string
	File              *string
}

// RestorePolicy represents a policy for restore operation.
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
func (r *RestoreFullRequest) Validate() error {
	if err := r.DestinationCuster.Validate(); err != nil {
		return err
	}
	if r.Directory == nil {
		return errors.New("restore directory is not specified")
	}
	return nil
}

func (r *RestoreFullRequest) ToRestoreRequest() RestoreRequest {
	return RestoreRequest{
		DestinationCuster: r.DestinationCuster,
		SourceStorage:     r.SourceStorage,
		Policy:            r.Policy,
		Directory:         r.Directory,
	}
}

// Validate validates the restore operation request.
func (r *RestoreIncrementalRequest) Validate() error {
	if err := r.DestinationCuster.Validate(); err != nil {
		return err
	}
	if r.File == nil {
		return errors.New("restore file is not specified")
	}
	return nil
}

func (r *RestoreIncrementalRequest) ToRestoreRequest() RestoreRequest {
	return RestoreRequest{
		DestinationCuster: r.DestinationCuster,
		SourceStorage:     r.SourceStorage,
		Policy:            r.Policy,
		File:              r.File,
	}
}

// Validate validates the restore operation request.
func (r *RestoreTimestampRequest) Validate() error {
	if err := r.DestinationCuster.Validate(); err != nil {
		return err
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

// String satisfies the fmt.Stringer interface.
func (r RestoreFullRequest) String() string {
	request, err := json.Marshal(r)
	if err != nil {
		return err.Error()
	}
	return string(request)
}

// String satisfies the fmt.Stringer interface.
func (r RestoreIncrementalRequest) String() string {
	request, err := json.Marshal(r)
	if err != nil {
		return err.Error()
	}
	return string(request)
}

// String satisfies the fmt.Stringer interface.
func (r RestoreTimestampRequest) String() string {
	request, err := json.Marshal(r)
	if err != nil {
		return err.Error()
	}
	return string(request)
}
