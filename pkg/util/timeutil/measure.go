package timeutil

import "time"

func MeasureDuration(f func() error) (time.Duration, error) {
	startTime := time.Now()
	err := f()
	return time.Since(startTime), err
}
