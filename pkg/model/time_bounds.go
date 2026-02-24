package model

import (
	"errors"
	"fmt"
	"time"
)

// TimeBounds represents a period of time between two timestamps.
type TimeBounds struct {
	FromTime *time.Time
	ToTime   *time.Time
}

// NewTimeBounds creates a new TimeBounds using provided fromTime and toTime values.
func NewTimeBounds(fromTime, toTime *time.Time) (TimeBounds, error) {
	if fromTime != nil && toTime != nil && fromTime.After(*toTime) {
		return TimeBounds{}, errors.New("fromTime must be less than or equal to toTime")
	}
	return TimeBounds{FromTime: fromTime, ToTime: toTime}, nil
}

// Contains verifies if the given value lies within FromTime (inclusive) and ToTime (inclusive).
func (tb TimeBounds) Contains(value time.Time) bool {
	if tb.FromTime != nil && value.Before(*tb.FromTime) {
		return false
	}

	if tb.ToTime != nil && value.After(*tb.ToTime) {
		return false
	}

	return true
}

// String implements the Stringer interface.
func (tb TimeBounds) String() string {
	if tb.FromTime == nil && tb.ToTime == nil {
		return "NA"
	}

	from := "NA"
	if tb.FromTime != nil {
		from = tb.FromTime.Format("2006-01-02 15:04:05")
	}

	to := "NA"
	if tb.ToTime != nil {
		to = tb.ToTime.Format("2006-01-02 15:04:05")
	}

	return fmt.Sprintf("%s - %s", from, to)
}
