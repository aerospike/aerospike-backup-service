package dto

import (
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

// NewTimeBoundsFromString creates a TimeBounds from the string representation of
// time boundaries (string is given as epoch time millis).
func NewTimeBoundsFromString(from, to string) (model.TimeBounds, error) {
	fromTime, err := parseTimestamp(from, "from")
	if err != nil {
		return model.TimeBounds{}, err
	}

	toTime, err := parseTimestamp(to, "to")
	if err != nil {
		return model.TimeBounds{}, err
	}

	return model.NewTimeBounds(fromTime, toTime)
}

func parseTimestamp(value, field string) (*time.Time, error) {
	if len(value) == 0 {
		return nil, nil
	}

	intValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, err
	}

	if intValue < 0 {
		return nil, errValidationNegative(field, intValue)
	}

	return util.Ptr(time.UnixMilli(intValue)), nil
}
