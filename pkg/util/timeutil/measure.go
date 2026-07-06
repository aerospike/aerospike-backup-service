package timeutil

import "time"

func MeasureDuration(f func() error) (time.Duration, error) {
	startTime := time.Now()
	err := f()
	return time.Since(startTime), err
}

// MeasureDurationWithResult measures the duration of a function that returns a value and an error.
func MeasureDurationWithResult[T any](f func() (T, error)) (T, time.Duration, error) {
	startTime := time.Now()
	value, err := f()
	return value, time.Since(startTime), err
}
