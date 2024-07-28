package model

import (
	"errors"
	"strconv"
	"time"
)

// TimeBounds represents a period of time between two timestamps.
type TimeBounds struct {
	FromTime *time.Time
	ToTime   *time.Time
}

// NewTimeBounds creates a new TimeBounds using provided fromTime and toTime values.
func NewTimeBounds(fromTime, toTime *time.Time) (*TimeBounds, error) {
	if fromTime != nil && toTime != nil && fromTime.After(*toTime) {
		return nil, errors.New("fromTime should be less than toTime")
	}
	return &TimeBounds{FromTime: fromTime, ToTime: toTime}, nil
}

// NewTimeBoundsTo creates a new TimeBounds until the provided toTime.
func NewTimeBoundsTo(toTime time.Time) *TimeBounds {
	timeBounds, _ := NewTimeBounds(nil, &toTime) // validation only make sense with two parameters.
	return timeBounds
}

// NewTimeBoundsFrom creates a new TimeBounds from the provided fromTime.
func NewTimeBoundsFrom(fromTime time.Time) *TimeBounds {
	timeBounds, _ := NewTimeBounds(&fromTime, nil) // validation only makes sense with two parameters.
	return timeBounds
}

// NewTimeBoundsFromString creates a TimeBounds from the string representation of
// time boundaries (string is given as epoch time millis).
func NewTimeBoundsFromString(from, to string) (*TimeBounds, error) {
	fromTime, err := parseTimestamp(from)
	if err != nil {
		return nil, err
	}
	toTime, err := parseTimestamp(to)
	if err != nil {
		return nil, err
	}
	return NewTimeBounds(fromTime, toTime)
}

func parseTimestamp(value string) (*time.Time, error) {
	if len(value) == 0 {
		return nil, nil
	}
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, err
	}
	result := time.UnixMilli(i)
	return &result, nil
}

// Contains verifies if the given value lies within FromTime (inclusive) and ToTime (exclusive).
func (tb *TimeBounds) Contains(value time.Time) bool {
	if tb.FromTime != nil && value.Before(*tb.FromTime) {
		return false
	}

	if tb.ToTime != nil && value.After(*tb.ToTime) {
		return false
	}

	return true
}
