package model

type StatusFilter struct {
	isExclude bool
	fields    map[JobStatus]struct{}
}

func NewStatusFilter(statuses []JobStatus, isExclude bool) StatusFilter {
	fieldMap := make(map[JobStatus]struct{}, len(statuses))
	for _, s := range statuses {
		fieldMap[s] = struct{}{}
	}

	return StatusFilter{
		fields:    fieldMap,
		isExclude: isExclude,
	}
}

func (sf StatusFilter) Matches(status JobStatus) bool {
	_, found := sf.fields[status]

	// If the filter is in "exclude" mode, we match if the status is NOT in the excluded set.
	if sf.isExclude {
		// true if the status is NOT excluded
		return !found
	}

	// true if the status IS included
	return found
}
