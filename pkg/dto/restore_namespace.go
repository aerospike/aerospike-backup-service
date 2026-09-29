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
	// A name follows the Aerospike naming rules: at most 31 bytes of Latin letters, digits,
	// "_", "-" and "$", and not the reserved name "null".
	Source NamespaceName `json:"source,omitempty" example:"source-ns" validate:"required"`
	// Name of the destination namespace to restore data into.
	// A name follows the Aerospike naming rules: at most 31 bytes of Latin letters, digits,
	// "_", "-" and "$", and not the reserved name "null".
	Destination NamespaceName `json:"destination,omitempty" example:"destination-ns" validate:"required"`
}

// Validate validates the restore namespace.
func (n *RestoreNamespace) Validate() error {
	if err := errValidationInvalidName("source", string(n.Source), n.Source.Validate()); err != nil {
		return err
	}

	return errValidationInvalidName("destination", string(n.Destination), n.Destination.Validate())
}

func (n *RestoreNamespace) ToModel() *model.RestoreNamespace {
	if n == nil {
		return nil
	}

	return &model.RestoreNamespace{
		Source:      string(n.Source),
		Destination: string(n.Destination),
	}
}
