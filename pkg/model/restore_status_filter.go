package model

// StatusFilter defines a filter for RestoreState values.
type StatusFilter struct {
	fields    map[RestoreState]struct{}
	isExclude bool
}

func NewStatusFilter(statuses []RestoreState, isExclude bool) StatusFilter {
	fieldMap := make(map[RestoreState]struct{}, len(statuses))
	for _, s := range statuses {
		fieldMap[s] = struct{}{}
	}

	return StatusFilter{
		fields:    fieldMap,
		isExclude: isExclude,
	}
}

// Matches returns true if the given status matches the filter criteria.
// If no statuses are defined in the filter, all statuses are considered a match.
// In include mode, it returns true if the status IS in the included set.
// In exclude mode, it returns true if the status is NOT in the excluded set.
func (sf StatusFilter) Matches(status RestoreState) bool {
	if len(sf.fields) == 0 {
		return true // No filters defined — match everything.
	}

	_, found := sf.fields[status]
	return sf.isExclude != found
}
