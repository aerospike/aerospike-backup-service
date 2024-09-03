package model

// RestoreNamespace specifies an alternative namespace name for the restore
// operation, where Source is the original namespace name and Destination is
// the namespace name to which the backup data is to be restored.
//
// @Description RestoreNamespace specifies an alternative namespace name for the restore
// @Description operation.
type RestoreNamespace struct {
	// Original namespace name.
	Source *string
	// Destination namespace name.
	Destination *string
}
