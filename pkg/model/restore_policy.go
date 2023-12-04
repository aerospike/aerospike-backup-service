package model

import "errors"

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

// Validate validates the restore policy.
func (p *RestorePolicy) Validate() error {
	if p == nil {
		return errors.New("restore policy is not specified")
	}
	return nil
}
