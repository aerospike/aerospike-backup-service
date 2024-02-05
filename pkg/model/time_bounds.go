package model

import (
	"errors"
	"log/slog"
	"strconv"
)

type TimeBounds struct {
	FromTime *int64
	ToTime   *int64
}

// NewTimeBounds creates new TimeBounds using provided fromTime and toTime values.
func NewTimeBounds(fromTime, toTime *int64) (*TimeBounds, error) {
	if fromTime != nil && *fromTime < 0 {
		return nil, errors.New("fromTime should be positive or zero")
	}
	if toTime != nil && *toTime <= 0 {
		return nil, errors.New("toTime should be positive")
	}
	if fromTime != nil && toTime != nil && *fromTime >= *toTime {
		return nil, errors.New("fromTime should be less than toTime")
	}
	return &TimeBounds{FromTime: fromTime, ToTime: toTime}, nil
}

// NewTimeBoundsTo creates new TimeBounds until provided toTime.
func NewTimeBoundsTo(toTime int64) (*TimeBounds, error) {
	return NewTimeBounds(nil, &toTime)
}

// NewTimeBoundsFrom creates new TimeBounds from provided fromTime.
func NewTimeBoundsFrom(toTime int64) (*TimeBounds, error) {
	return NewTimeBounds(nil, &toTime)
}

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

func parseTimestamp(value string) (*int64, error) {
	if len(value) == 0 {
		return nil, nil
	}
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

// Contains verifies if given value lies within FromTime (inclusive) and ToTime (exclusive).
func (tb *TimeBounds) Contains(value int64) bool {
	if tb.FromTime != nil && value < *tb.FromTime {
		slog.Debug("contains failed <", "from", *tb.FromTime, "value", value)
		return false
	}
	if tb.ToTime != nil && value >= *tb.ToTime {
		slog.Debug("contains failed >", "to", *tb.ToTime, "value", value)
		return false
	}
	slog.Debug("contains passed", "to", tb.ToTime, "from", tb.FromTime, "value", value)
	return true
}
