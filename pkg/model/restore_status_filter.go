package model

type StatusFilter struct {
	include map[JobStatus]bool
	exclude map[JobStatus]bool
}

func NewStatusFilter(include, exclude map[JobStatus]bool) StatusFilter {
	return StatusFilter{
		include: include,
		exclude: exclude,
	}
}

func (sf StatusFilter) Matches(status JobStatus) bool {
	if sf.exclude[status] {
		return false
	}
	if len(sf.include) > 0 {
		return sf.include[status]
	}

	return true
}
