package model

import "errors"

// RestorePolicy represents a policy for the restore operation.
type RestorePolicy struct {
	Parallel           *uint32           `json:"parallel,omitempty"`
	NoRecords          *bool             `json:"no-records,omitempty"`
	NoIndexes          *bool             `json:"no-indexes,omitempty"`
	NoUdfs             *bool             `json:"no-udfs,omitempty"`
	Timeout            *uint32           `json:"timeout,omitempty"`
	DisableBatchWrites *bool             `json:"disable-batch-writes,omitempty"`
	MaxAsyncBatches    *uint32           `json:"max-async-batches,omitempty"`
	BatchSize          *uint32           `json:"batch-size,omitempty"`
	Namespace          *RestoreNamespace `json:"namespace,omitempty"`
	SetList            []string          `json:"set-list,omitempty"`
	BinList            []string          `json:"bin-list,omitempty"`
	Replace            *bool             `json:"replace,omitempty"`
	Unique             *bool             `json:"unique,omitempty"`
	NoGeneration       *bool             `json:"no-generation,omitempty"`
	Bandwidth          *uint64           `json:"bandwidth,omitempty"`
	Tps                *uint32           `json:"tps,omitempty"`
}

// RestoreNamespace specifies an alternative namespace name for the restore
// operation, where Source is the original namespace name and Destination is
// the namespace name to which the backup data is to be restored.
type RestoreNamespace struct {
	Source      *string `json:"source,omitempty"`
	Destination *string `json:"destination,omitempty"`
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
