package dto

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/pkg/model"
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
	intValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, err
	}
	if intValue < 0 {
		return nil, fmt.Errorf("timestamp should be positive or zero, got %d", intValue)
	}
	result := time.UnixMilli(intValue)
	return &result, nil
}

func MapTimeBoundsFromDTO(dto TimeBounds) model.TimeBounds {
	return model.TimeBounds{
		FromTime: dto.FromTime,
		ToTime:   dto.ToTime,
	}
}
