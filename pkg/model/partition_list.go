package model

import (
	"fmt"
	"strconv"

	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go"
)

// TODO(BKRS-453): open review findings for the partition-list validation.
//   - A list with more entries than backup-policy.parallel passes validation, but backup-go's
//     splitPartitions rejects it at scan time. Check len(list) <= parallel in
//     dto.BackupRoutine.ToModel, next to ValidateBackupPolicyParallelism.
//   - Digest entries (base64 record digests) are accepted as "the partition holding the digest",
//     but the partition-list doc comment and OpenAPI describe only IDs and ranges. Document or reject.
//   - dto fromModel writes the list in canonical form ("0-1" -> "0", "007" -> "7",
//     digest -> partition ID), so GET /v1/config and the saved config differ from the input.
//   - Whitespace used to be trimmed and is now rejected; one routine with "0, 100" stops the
//     whole config from loading. Needs a release note.
//   - The backup-go parse error repeats itself, and an empty entry is reported as a digest error.
//     Wrap it with a clearer message.
//   - The list is parsed twice: dto.PartitionList.Validate and dto.BackupRoutine.ToModel.
//   - errValidationPartitionList lives in dto/backup_routine.go instead of dto/validate_errors.go.
//   - The partition-list doc comment in dto/backup_routine.go does not mention the whitespace
//     and overlap rules; update it and run make docs.
//   - TestMakeBackupConfig_NoPartitionList hard-codes 4096; compare against
//     backup.NewDefaultBackupConfig().PartitionFilters instead.

// PartitionRange is Count consecutive partitions starting at partition Begin.
type PartitionRange struct {
	Begin int
	Count int
}

// NewPartitionRange returns the range a partition filter selects.
func NewPartitionRange(f *as.PartitionFilter) PartitionRange {
	return PartitionRange{Begin: f.Begin, Count: f.Count}
}

// End returns the partition after the last one in the range.
func (r PartitionRange) End() int {
	return r.Begin + r.Count
}

// String formats the range in the partition-list syntax: "<begin>-<count>", or "<begin>" for a
// single partition.
func (r PartitionRange) String() string {
	if r.Count == 1 {
		return strconv.Itoa(r.Begin)
	}

	return fmt.Sprintf("%d-%d", r.Begin, r.Count)
}

// ToPartitionFilter returns a new partition filter selecting the range.
func (r PartitionRange) ToPartitionFilter() *as.PartitionFilter {
	return as.NewPartitionFilterByRange(r.Begin, r.Count)
}

// PartitionList is the disjoint partition ranges a routine backs up.
// An empty list means all partitions.
type PartitionList []PartitionRange

// ToPartitionFilters returns new partition filters selecting the list's partitions.
// A scan records its progress in the filters it is given, so every scan needs filters of its own.
func (l PartitionList) ToPartitionFilters() []*as.PartitionFilter {
	if len(l) == 0 {
		return []*as.PartitionFilter{backup.NewPartitionFilterAll()}
	}

	filters := make([]*as.PartitionFilter, len(l))
	for i, r := range l {
		filters[i] = r.ToPartitionFilter()
	}

	return filters
}
