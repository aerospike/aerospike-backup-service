package model

import (
	"encoding/json"
)

// RestoreRequest represents a restore operation request.
type RestoreRequest struct {
	DestinationCuster *AerospikeCluster `json:"destination,omitempty"`
	Policy            *RestorePolicy    `json:"policy,omitempty"`
	SourceStorage     *Storage          `json:"source,omitempty"`
}

// RestoreRequestInternal is only used internally to pass data to restore library.
type RestoreRequestInternal struct {
	RestoreRequest
	File *string
	Dir  *string
}

// RestoreTimestampRequest represents a restore by timestamp operation request.
type RestoreTimestampRequest struct {
	DestinationCuster *AerospikeCluster `json:"destination,omitempty"`
	Policy            *RestorePolicy    `json:"policy,omitempty"`
	Time              int64             `json:"time,omitempty" format:"int64"`
	Routine           string            `json:"routine,omitempty"`
}

// RestorePolicy represents a policy for restore operation.
type RestorePolicy struct {
	Parallel           *uint32  `json:"parallel,omitempty"`
	NoRecords          *bool    `json:"no-records,omitempty"`
	NoIndexes          *bool    `json:"no-indexes,omitempty"`
	NoUdfs             *bool    `json:"no-udfs,omitempty"`
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

// String satisfies the fmt.Stringer interface.
func (r RestoreRequest) String() string {
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
