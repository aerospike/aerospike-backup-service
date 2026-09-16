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
// Entries are compared by value, never by pointer.
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

// ChangedRoutines returns the sorted names of every routine that differs between two
// backup configurations, including the routines only one of them holds. Diff reports what
// its receiver holds, so this runs it both ways; neither configuration is privileged, and
// they can be passed in either order.
//
// A routine holds its cluster, policy and storage directly rather than by name, so editing
// any of those is reported here as a change to every routine that uses it. Nothing has to
// track which routine refers to what.
func ChangedRoutines(a, b *BackupConfig) []string {
	changed := make(map[string]struct{})
	for name := range a.Diff(b).BackupRoutines {
		changed[name] = struct{}{}
	}

	for name := range b.Diff(a).BackupRoutines {
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
