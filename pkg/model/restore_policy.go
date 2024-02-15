package model

import "errors"

type RestorePolicy struct {
	Parallel           *uint32           `json:"parallel,omitempty" example:"8"`
	NoRecords          *bool             `json:"no-records,omitempty"`
	NoIndexes          *bool             `json:"no-indexes,omitempty"`
	NoUdfs             *bool             `json:"no-udfs,omitempty"`
	Timeout            *uint32           `json:"timeout,omitempty" example:"1000"`
	DisableBatchWrites *bool             `json:"disable-batch-writes,omitempty"`
	MaxAsyncBatches    *uint32           `json:"max-async-batches,omitempty" example:"32"`
	BatchSize          *uint32           `json:"batch-size,omitempty" example:"128"`
	Namespace          *RestoreNamespace `json:"namespace,omitempty"`
	SetList            []string          `json:"set-list,omitempty" example:"set1,set2"`
	BinList            []string          `json:"bin-list,omitempty" example:"bin1,bin2"`
	Replace            *bool             `json:"replace,omitempty"`
	Unique             *bool             `json:"unique,omitempty"`
	NoGeneration       *bool             `json:"no-generation,omitempty"`
	Bandwidth          *uint64           `json:"bandwidth,omitempty" example:"50000"`
	Tps                *uint32           `json:"tps,omitempty" example:"4000"`
}

type RestoreNamespace struct {
	Source      *string `json:"source,omitempty" example:"source-ns" validate:"required"`
	Destination *string `json:"destination,omitempty" example:"destination-ns" validate:"required"`
}

// Validate validates the restore policy.
func (p *RestorePolicy) Validate() error {
	if p == nil {
		return errors.New("restore policy is not specified")
	}
	if p.Namespace != nil { // namespace is optional.
		if err := p.Namespace.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate validates the restore namespace.
func (n *RestoreNamespace) Validate() error {
	if n.Source == nil {
		return errors.New("source namespace is not specified")
	}
	if n.Destination == nil {
		return errors.New("destination namespace is not specified")
	}
	return nil
}
