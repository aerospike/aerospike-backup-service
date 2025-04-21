package service

import (
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"go.uber.org/mock/gomock"
)

type timeBounderMatcher struct {
	expectedFromTime time.Time
}

func (m timeBounderMatcher) Matches(x any) bool {
	if timeBounds, ok := x.(model.TimeBounds); ok {
		return timeBounds.FromTime != nil && timeBounds.FromTime.Equal(m.expectedFromTime)
	}
	return false
}

func (m timeBounderMatcher) String() string {
	return fmt.Sprintf("TimeBounds with FromTime=%v", m.expectedFromTime)
}

func newTimeBoundsFromTimeMatcher(fromTime time.Time) gomock.Matcher {
	return timeBounderMatcher{expectedFromTime: fromTime}
}

type timeMatcher struct {
	expectedFromTime time.Time
}

func (m timeMatcher) Matches(x any) bool {
	if t, ok := x.(time.Time); ok {
		return t.Equal(m.expectedFromTime)
	}
	return false
}

func (m timeMatcher) String() string {
	return m.expectedFromTime.String()
}

func newTimeMatcher(fromTime time.Time) gomock.Matcher {
	return timeMatcher{expectedFromTime: fromTime}
}
