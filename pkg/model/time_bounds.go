package model

import (
	"errors"
	"math"
	"strconv"
)

type TimeBounds struct {
	FromTime int64
	ToTime   int64
}

func NewTimeBounds(fromTime, toTime int64) *TimeBounds {
	return &TimeBounds{FromTime: fromTime, ToTime: toTime}
}

func NewTimeBoundsTo(toTime int64) *TimeBounds {
	return &TimeBounds{FromTime: 0, ToTime: toTime}
}

func NewTimeBoundsFromString(from, to string) (*TimeBounds, error) {
	fromTime, err := parseTimestamp(from, 0)
	if err != nil {
		return nil, err
	}
	toTime, err := parseTimestamp(to, math.MaxInt64)
	if err != nil {
		return nil, err
	}
	if toTime <= fromTime {
		return nil, errors.New("invalid time range: toTime should be greater than fromTime")
	}
	if toTime <= 0 || fromTime < 0 {
		return nil, errors.New("requested time filters must be positive")
	}
	return &TimeBounds{FromTime: fromTime, ToTime: toTime}, nil
}

func parseTimestamp(value string, defaultValue int64) (int64, error) {
	if len(value) == 0 {
		return defaultValue, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func (tb *TimeBounds) Contains(value int64) bool {
	return tb.FromTime <= value && value < tb.ToTime
}
