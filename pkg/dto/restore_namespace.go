package dto

import (
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// RestoreNamespace specifies an alternative namespace name for the restore
// operation, where Source is the original namespace name and Destination is
// the namespace name to which the backup data is to be restored.
//
// @Description RestoreNamespace specifies an alternative namespace name for the restore operation.
type RestoreNamespace struct {
	// Original namespace name.
	// This field is required as a safeguard to ensure intentional namespace remapping.
	Source *string `json:"source,omitempty" example:"source-ns" validate:"required"`
	// Destination Name of the destination namespace to restore data into.
	Destination *string `json:"destination,omitempty" example:"destination-ns" validate:"required"`
}

// Validate validates the restore namespace.
func (n *RestoreNamespace) Validate() error {
	if n.Source == nil {
		return errValidationEmptyField("source")
	}

	if n.Destination == nil {
		return errValidationEmptyField("destination")
	}

	return nil
}

func (n *RestoreNamespace) ToModel() *model.RestoreNamespace {
	if n == nil {
		return nil
	}

	return &model.RestoreNamespace{
		Source:      n.Source,
		Destination: n.Destination,
	}
}
