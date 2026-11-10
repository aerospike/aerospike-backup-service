package service

import (
	"time"

	"go.uber.org/mock/gomock"
)

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
