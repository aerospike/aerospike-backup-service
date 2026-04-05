package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusFilter_Matches(t *testing.T) {
	statuses := []RestoreState{
		RestoreRunning,
		RestoreSuccess,
	}

	// will allow running and done jobs to pass through the filter
	inclusiveFilter := NewStatusFilter(statuses, false)
	assert.True(t, inclusiveFilter.Matches(RestoreRunning))
	assert.True(t, inclusiveFilter.Matches(RestoreSuccess))
	assert.False(t, inclusiveFilter.Matches(RestoreFailure))

	// will block running and done jobs from passing through the filter
	exclusiveFilter := NewStatusFilter(statuses, true)
	assert.False(t, exclusiveFilter.Matches(RestoreRunning))
	assert.False(t, exclusiveFilter.Matches(RestoreSuccess))
	assert.True(t, exclusiveFilter.Matches(RestoreFailure))

	// will match all job statuses regardless of isExclude value
	emptyInclude := NewStatusFilter([]RestoreState{}, false)
	assert.True(t, emptyInclude.Matches(RestoreRunning))

	emptyExclude := NewStatusFilter([]RestoreState{}, true)
	assert.True(t, emptyExclude.Matches(RestoreRunning))
}
