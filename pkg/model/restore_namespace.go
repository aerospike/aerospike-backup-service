package model

import "fmt"

// RestoreNamespace specifies an alternative namespace name for the restore
// operation, where Source is the original namespace name and Destination is
// the namespace name to which the backup data is to be restored.
//
// @Description RestoreNamespace specifies an alternative namespace name for the restore
// @Description operation.
type RestoreNamespace struct {
	// Original namespace name.
	Source *string `json:"source,omitempty" example:"source-ns" validate:"required"`
	// Destination namespace name.
	Destination *string `json:"destination,omitempty" example:"destination-ns" validate:"required"`
}

// Validate validates the restore namespace.
func (n *RestoreNamespace) Validate() error {
	if n.Source == nil {
		return fmt.Errorf("source namespace is not specified")
	}

	if n.Destination == nil {
		return fmt.Errorf("destination namespace is not specified")
	}

	return nil
}
