package model

// RestoreNamespace specifies an alternative namespace name for the restore
// operation, where Source is the original namespace name and Destination is
// the namespace name to which the backup data is to be restored.
type RestoreNamespace struct {
	// Original namespace name.
	Source string
	// Destination namespace name.
	Destination string
}

// DestinationOr returns the namespace records from source are written to: the configured
// Destination when remapping is set, otherwise source itself. A nil remapping (no Namespace
// configured on the restore policy) behaves the same as one with no Destination set.
func (n *RestoreNamespace) DestinationOr(source string) string {
	if n != nil && n.Destination != "" {
		return n.Destination
	}

	return source
}
