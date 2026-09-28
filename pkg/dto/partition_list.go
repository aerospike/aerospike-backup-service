package dto

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
)

// PartitionList is the raw partition-list value of a backup routine: comma-separated partition
// IDs ("100") and ranges ("<begin>-<count>", e.g. "100-50").
//
// The value is parsed with backup.ParsePartitionFilterListString, the parser backup-go documents
// for this format. On top of that, the list must stay within the partition count and must not
// select a partition twice.
type PartitionList string

// newPartitionList formats ranges in the partition-list syntax.
func newPartitionList(ranges model.PartitionList) PartitionList {
	entries := make([]string, len(ranges))
	for i, r := range ranges {
		entries[i] = r.String()
	}

	return PartitionList(strings.Join(entries, ","))
}

// Validate returns an error unless every backup of a routine can use the list.
func (p PartitionList) Validate() error {
	_, err := p.toModel()
	return err
}

// toModel parses the list into ranges, in the order they are listed.
func (p PartitionList) toModel() (model.PartitionList, error) {
	if p == "" {
		return nil, nil
	}
	// backup-go rejects whitespace too, but its error does not show where it is.
	if strings.ContainsFunc(string(p), unicode.IsSpace) {
		return nil, errors.New("must not contain whitespace")
	}

	// A digest entry selects the partition holding that digest, which does not depend on the
	// namespace, so the routine's namespaces need not be known here.
	filters, err := backup.ParsePartitionFilterListString("", string(p))
	if err != nil {
		return nil, err
	}

	ranges := make(model.PartitionList, len(filters))
	for i, f := range filters {
		ranges[i] = model.NewPartitionRange(f)
		if ranges[i].End() > backup.MaxPartitions {
			return nil, fmt.Errorf("%s exceeds the last partition %d", ranges[i], backup.MaxPartitions-1)
		}
	}

	if err := validateDisjoint(ranges); err != nil {
		return nil, err
	}

	return ranges, nil
}

// validateDisjoint reports the first two ranges that select the same partition.
func validateDisjoint(ranges model.PartitionList) error {
	sorted := slices.SortedFunc(slices.Values(ranges), func(a, b model.PartitionRange) int {
		return cmp.Compare(a.Begin, b.Begin)
	})

	for i := 1; i < len(sorted); i++ {
		prev, next := sorted[i-1], sorted[i]
		if next.Begin < prev.End() {
			return fmt.Errorf("%s overlaps %s", next, prev)
		}
	}

	return nil
}
