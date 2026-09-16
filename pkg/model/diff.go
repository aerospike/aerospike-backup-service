package model

import (
	"maps"
	"reflect"
	"slices"
)

// Diff returns a backup configuration holding just the entries bc has that other does not,
// or holds under the same name with a different value - a cluster, storage, policy, routine
// or secret agent that is new or configured differently.
//
// The comparison is one-directional: an entry other has and bc does not is absent from the
// result, and running Diff the other way round is what reports it.
//
// Entries are compared by value, never by pointer. Every configuration change round trips
// through the DTO layer, which allocates fresh entries even for the parts nobody touched,
// so every pointer differs on every change. The comparison is deep rather than a list of
// the fields that matter, because it can only ever report too much: a caller that reacts to
// a change it did not need to pays for a redundant reaction, where a field an explicit list
// forgets leaves that caller acting on configuration the service no longer holds.
func (bc *BackupConfig) Diff(other *BackupConfig) *BackupConfig {
	delta := newBackupConfig()
	if bc == nil {
		return delta
	}

	if other == nil {
		other = newBackupConfig()
	}

	diffEntries(delta.AerospikeClusters, bc.AerospikeClusters, other.AerospikeClusters)
	diffEntries(delta.Storage, bc.Storage, other.Storage)
	diffEntries(delta.BackupPolicies, bc.BackupPolicies, other.BackupPolicies)
	diffEntries(delta.BackupRoutines, bc.BackupRoutines, other.BackupRoutines)
	diffEntries(delta.SecretAgents, bc.SecretAgents, other.SecretAgents)

	return delta
}

// ChangedRoutines returns the sorted names of every routine that differs between bc and
// other, including the routines only one of them holds. It is symmetric, so the two
// configurations can be passed in either order.
//
// A routine holds its cluster, policy and storage directly rather than by name, so editing
// any of those is reported here as a change to every routine that uses it. Nothing has to
// track which routine refers to what.
func (bc *BackupConfig) ChangedRoutines(other *BackupConfig) []string {
	changed := make(map[string]struct{})
	for name := range bc.Diff(other).BackupRoutines {
		changed[name] = struct{}{}
	}

	for name := range other.Diff(bc).BackupRoutines {
		changed[name] = struct{}{}
	}

	return slices.Sorted(maps.Keys(changed))
}

// diffEntries copies into delta every entry of from that to does not hold under the same
// name with an equal value.
func diffEntries[T any](delta, from, to map[string]T) {
	for name, entry := range from {
		if !reflect.DeepEqual(to[name], entry) {
			delta[name] = entry
		}
	}
}
