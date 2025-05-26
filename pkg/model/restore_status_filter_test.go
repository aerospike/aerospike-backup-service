package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusFilter_Matches(t *testing.T) {
	statuses := []JobStatus{
		JobStatusRunning,
		JobStatusDone,
	}

	// will allow running and done jobs to pass through the filter
	inclusiveFilter := NewStatusFilter(statuses, false)
	assert.True(t, inclusiveFilter.Matches(JobStatusRunning))
	assert.True(t, inclusiveFilter.Matches(JobStatusDone))
	assert.False(t, inclusiveFilter.Matches(JobStatusFailed))

	// will block running and done jobs from passing through the filter
	exclusiveFilter := NewStatusFilter(statuses, true)
	assert.False(t, exclusiveFilter.Matches(JobStatusRunning))
	assert.False(t, exclusiveFilter.Matches(JobStatusDone))
	assert.True(t, exclusiveFilter.Matches(JobStatusFailed))

	// will not match any job status
	emptyInclude := NewStatusFilter([]JobStatus{}, false)
	assert.False(t, emptyInclude.Matches(JobStatusRunning))

	// will match all job statuses
	emptyExclude := NewStatusFilter([]JobStatus{}, true)
	assert.True(t, emptyExclude.Matches(JobStatusRunning))
}
